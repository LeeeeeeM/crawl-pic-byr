package crawler

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"crawl-pic/backend/internal/models"
	"crawl-pic/backend/internal/repository"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

type BaiduIndexService struct {
	repo      *repository.Repository
	assetsDir string
}

type canvasCapture struct {
	OK      bool   `json:"ok"`
	DataURL string `json:"dataUrl"`
	Error   string `json:"error"`
}

type trendCanvasResult struct {
	SearchDataURL string `json:"searchDataUrl"`
	InfoDataURL   string `json:"infoDataUrl"`
	Error         string `json:"error"`
}

type baiduPageState struct {
	Href           string `json:"href"`
	OnPassportPage bool   `json:"onPassportPage"`
	HasLoginHint   bool   `json:"hasLoginHint"`
	HasChartCanvas bool   `json:"hasChartCanvas"`
}

type periodState struct {
	Selected string `json:"selected"`
}

func NewBaiduIndex(repo *repository.Repository, assetsDir string) *BaiduIndexService {
	return &BaiduIndexService{repo: repo, assetsDir: assetsDir}
}

func (s *BaiduIndexService) Run(parent context.Context, jobID uuid.UUID, cfg models.BaiduIndexCrawlConfig) error {
	if err := s.repo.ValidateBaiduIndexConfig(cfg); err != nil {
		return err
	}

	wsURL, err := fetchWebSocketDebuggerURL(cfg.RemoteDebugURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 15*time.Minute)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	if cfg.Period == "" {
		cfg.Period = "90d"
	}
	if cfg.WaitAfterLoadMs <= 0 {
		cfg.WaitAfterLoadMs = 1800
	}

	targetURL := strings.TrimSpace(cfg.StartURL)
	if targetURL == "" {
		targetURL = buildBaiduTrendURL(cfg.Keyword)
	}

	if err := chromedp.Run(browserCtx,
		chromedp.Navigate(targetURL),
		chromedp.Sleep(1800*time.Millisecond),
	); err != nil {
		return err
	}
	if err := s.waitUntilTrendReady(browserCtx, targetURL, 1*time.Minute); err != nil {
		return err
	}

	// Baidu index defaults to 30d. For 30d we skip dropdown interaction.
	if cfg.Period != "30d" {
		if err := s.selectPeriod(browserCtx, cfg.Period); err != nil {
			return err
		}
		if err := s.waitPeriodApplied(browserCtx, cfg.Period, 30*time.Second); err != nil {
			return err
		}
	}
	if err := chromedp.Run(browserCtx, chromedp.Sleep(time.Duration(cfg.WaitAfterLoadMs)*time.Millisecond)); err != nil {
		return err
	}

	capture, currentURL, err := s.captureTrendCanvases(browserCtx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(capture.SearchDataURL) == "" || strings.TrimSpace(capture.InfoDataURL) == "" {
		if capture.Error == "" {
			capture.Error = "search/info canvas capture failed"
		}
		return fmt.Errorf("%s", capture.Error)
	}

	searchFileName, searchAssetURL, searchSize, err := s.saveCapture(cfg.Keyword, cfg.Period, "搜索", capture.SearchDataURL)
	if err != nil {
		return err
	}
	if cfg.MinImageBytes > 0 && int64(searchSize) < cfg.MinImageBytes {
		return fmt.Errorf("captured search image is smaller than minImageBytes")
	}

	infoFileName, infoAssetURL, infoSize, err := s.saveCapture(cfg.Keyword, cfg.Period, "咨询", capture.InfoDataURL)
	if err != nil {
		return err
	}
	if cfg.MinImageBytes > 0 && int64(infoSize) < cfg.MinImageBytes {
		return fmt.Errorf("captured consult image is smaller than minImageBytes")
	}

	title := fmt.Sprintf("Baidu Index %s (%s)", cfg.Keyword, cfg.Period)
	content := fmt.Sprintf(
		"keyword=%s\nperiod=%s\nsearchFile=%s\nconsultFile=%s",
		cfg.Keyword,
		cfg.Period,
		searchFileName,
		infoFileName,
	)

	qctx, qcancel := repository.WithTimeout(ctx)
	postID, err := s.repo.UpsertPost(qctx, jobID, currentURL, title, content)
	qcancel()
	if err != nil {
		return err
	}

	ictx, icancel := repository.WithTimeout(ctx)
	err = s.repo.InsertPhoto(ictx, jobID, postID, searchAssetURL, cfg.Keyword+"-搜索")
	icancel()
	if err != nil {
		return err
	}

	ictx2, icancel2 := repository.WithTimeout(ctx)
	err = s.repo.InsertPhoto(ictx2, jobID, postID, infoAssetURL, cfg.Keyword+"-咨询")
	icancel2()
	if err != nil {
		return err
	}

	return nil
}

func (s *BaiduIndexService) waitUntilTrendReady(ctx context.Context, targetURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var last baiduPageState
	for {
		state, err := s.readPageState(ctx)
		if err != nil {
			return err
		}
		last = state
		if state.HasChartCanvas && !state.OnPassportPage {
			return nil
		}

		if !state.OnPassportPage && strings.Contains(state.Href, "index.baidu.com") && !strings.Contains(state.Href, "/trend/") {
			_ = chromedp.Run(ctx, chromedp.Navigate(targetURL))
		}

		if time.Now().After(deadline) {
			break
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(2*time.Second)); err != nil {
			return err
		}
	}

	if last.OnPassportPage || last.HasLoginHint {
		return fmt.Errorf("login required: please complete login in Chrome within 1 minute")
	}
	return fmt.Errorf("trend chart is not ready within 1 minute")
}

func (s *BaiduIndexService) readPageState(ctx context.Context) (baiduPageState, error) {
	var state baiduPageState
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const href = location.href || '';
			const text = (document.body && document.body.innerText) ? document.body.innerText : '';
			const canvasList = Array.from(document.querySelectorAll('canvas'));
			const hasChartCanvas = canvasList.some((c) => {
				const r = c.getBoundingClientRect();
				return (c.width || 0) > 200 && (c.height || 0) > 100 && r.width > 200 && r.height > 100;
			});
			const hasLoginHint = href.includes('passport.baidu.com') ||
				/登录|请登录|登录后/.test(text) ||
				!!document.querySelector('a[href*="passport.baidu.com"], .login-btn, .userbar-login');
			return {
				href,
				onPassportPage: href.includes('passport.baidu.com'),
				hasLoginHint,
				hasChartCanvas
			};
		})()`, &state),
	)
	if err != nil {
		return baiduPageState{}, err
	}
	return state, nil
}

func (s *BaiduIndexService) selectPeriod(ctx context.Context, period string) error {
	targetLabel, err := baiduPeriodLabel(period)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		var ok bool
		js := fmt.Sprintf(`(() => {
				const label = %q;
				const isVisible = (el) => {
					if (!el) return false;
					const r = el.getBoundingClientRect();
					const st = window.getComputedStyle(el);
					return r.width > 0 && r.height > 0 && st.visibility !== 'hidden' && st.display !== 'none';
				};
				const isPeriodText = (txt) => /近7天|近30天|近90天|近半年|全部|自定义/.test((txt || '').replace(/\s+/g, ''));

				// Locate search chart card only.
				const cards = Array.from(document.querySelectorAll('.index-trend-content'));
				const searchCard = cards.find((card) => {
					const text = (card.textContent || '').replace(/\s+/g, '');
					return text.includes('搜索指数') && !!card.querySelector('.index-trend-chart');
				}) || null;

				let trigger = null;
				if (searchCard) {
					const btns = Array.from(searchCard.querySelectorAll('.index-dropdown-list .veui-button, .index-dropdown-list button'));
					trigger = btns.find((b) => isVisible(b) && isPeriodText(b.textContent || '')) || null;
				}
				if (!trigger) return false;

				try { trigger.scrollIntoView({ block: 'center', inline: 'nearest' }); } catch (_) {}
				try { trigger.click(); } catch (_) {}

				const overlays = Array.from(document.querySelectorAll('.veui-overlay-box.index-dropdown-list-overlay-box, .veui-overlay-box'));
				const visibleOverlays = overlays.filter((el) => isVisible(el) && /近7天|近30天|近90天|近半年|全部|自定义/.test((el.textContent || '')));
				const visibleOverlay = visibleOverlays.length ? visibleOverlays[visibleOverlays.length - 1] : null;
				if (!visibleOverlay) return false;

				let target = Array.from(visibleOverlay.querySelectorAll('.list-item, [role="option"]'))
					.find((el) => isVisible(el) && (el.textContent || '').replace(/\s+/g, '') === label);
				if (!target) {
					target = Array.from(visibleOverlay.querySelectorAll('.list-item, [role="option"]'))
						.find((el) => isVisible(el) && (el.textContent || '').replace(/\s+/g, '').includes(label));
				}
				if (!target) return false;

				try { target.scrollIntoView({ block: 'center', inline: 'nearest' }); } catch (_) {}
				try { target.click(); } catch (_) {}
				return true;
			})()`, targetLabel)
		err := chromedp.Run(ctx,
			chromedp.Evaluate(js, &ok),
		)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(1200*time.Millisecond)); err != nil {
			return err
		}
	}
	return fmt.Errorf("cannot find period selector for %s within 30s", targetLabel)
}

func (s *BaiduIndexService) waitPeriodApplied(ctx context.Context, period string, timeout time.Duration) error {
	expected, err := baiduPeriodLabel(period)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(timeout)
	for {
		state, err := s.readPeriodState(ctx)
		if err != nil {
			return err
		}
		if strings.Contains(state.Selected, expected) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("period switch not applied: expected %s, got %s", expected, state.Selected)
		}
		if err := chromedp.Run(ctx, chromedp.Sleep(1200*time.Millisecond)); err != nil {
			return err
		}
	}
}

func baiduPeriodLabel(period string) (string, error) {
	switch period {
	case "7d":
		return "近7天", nil
	case "30d":
		return "近30天", nil
	case "90d":
		return "近90天", nil
	case "180d":
		return "近半年", nil
	case "all":
		return "全部", nil
	default:
		return "", fmt.Errorf("unsupported period: %s", period)
	}
}

func (s *BaiduIndexService) readPeriodState(ctx context.Context) (periodState, error) {
	var state periodState
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`(() => {
			const isVisible = (el) => {
				if (!el) return false;
				const r = el.getBoundingClientRect();
				const st = window.getComputedStyle(el);
				return r.width > 0 && r.height > 0 && st.visibility !== 'hidden' && st.display !== 'none';
			};
			const isPeriodText = (txt) => /近7天|近30天|近90天|近半年|全部|自定义/.test((txt || '').replace(/\s+/g, ''));

			const cards = Array.from(document.querySelectorAll('.index-trend-content'));
			const searchCard = cards.find((card) => {
				const text = (card.textContent || '').replace(/\s+/g, '');
				return text.includes('搜索指数') && !!card.querySelector('.index-trend-chart');
			}) || null;
			const target = searchCard
				? Array.from(searchCard.querySelectorAll('.index-dropdown-list .veui-button, .index-dropdown-list button'))
					.find((b) => isVisible(b) && isPeriodText(b.textContent || '')) || null
				: null;
			return {
				selected: target ? (target.textContent || '').replace(/\s+/g, '') : ''
			};
		})()`, &state),
	)
	if err != nil {
		return periodState{}, err
	}
	return state, nil
}

func (s *BaiduIndexService) captureTrendCanvases(ctx context.Context) (trendCanvasResult, string, error) {
	var cap trendCanvasResult
	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Location(&currentURL),
		chromedp.Evaluate(`(() => {
			const cards = Array.from(document.querySelectorAll('.index-trend-content'));
			const chartCards = cards.filter((card) => card.querySelector('.index-trend-chart canvas'));
			const pickCanvasDataURL = (root) => {
				if (!root) return '';
				const list = Array.from(root.querySelectorAll('.index-trend-chart canvas'));
				const candidates = list
					.map((c) => ({
						canvas: c,
						w: c.width || 0,
						h: c.height || 0,
						r: c.getBoundingClientRect(),
					}))
					.filter((x) => x.w > 200 && x.h > 100 && x.r.width > 200 && x.r.height > 100)
					.sort((a, b) => (b.w * b.h) - (a.w * a.h));
				if (!candidates.length) return '';
				try {
					return candidates[0].canvas.toDataURL('image/png');
				} catch (_) {
					return '';
				}
			};

			// Primary: pick by text match.
			let searchCard = null;
			let infoCard = null;
			for (const card of chartCards) {
				const text = (card.textContent || '').replace(/\s+/g, '');
				if (!searchCard && text.includes('搜索指数')) {
					searchCard = card;
					continue;
				}
				if (!infoCard && (text.includes('资讯指数') || text.includes('咨询指数') || text.includes('资讯关注'))) {
					infoCard = card;
				}
			}
			// Fallback: pick first two chart cards.
			if (!searchCard && chartCards.length > 0) searchCard = chartCards[0];
			if (!infoCard && chartCards.length > 1) infoCard = chartCards[1];

			const searchDataUrl = pickCanvasDataURL(searchCard);
			const infoDataUrl = pickCanvasDataURL(infoCard);

			if (!searchDataUrl || !infoDataUrl) {
				return {
					searchDataUrl,
					infoDataUrl,
					error: 'failed to capture both search and consult canvases',
				};
			}

			return { searchDataUrl, infoDataUrl, error: '' };
		})()`, &cap),
	)
	if err != nil {
		return trendCanvasResult{}, "", err
	}
	return cap, currentURL, nil
}

func (s *BaiduIndexService) saveCapture(keyword, period, suffix, dataURL string) (string, string, int, error) {
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		return "", "", 0, fmt.Errorf("unexpected canvas payload format")
	}
	raw := strings.TrimPrefix(dataURL, prefix)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", "", 0, fmt.Errorf("decode canvas image failed: %w", err)
	}
	if len(decoded) == 0 {
		return "", "", 0, fmt.Errorf("captured image is empty")
	}

	dir := filepath.Join(s.assetsDir, "baidu-index")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", 0, err
	}

	ts := time.Now().Format("20060102_150405")
	name := fmt.Sprintf("%s_%s_%s_%s.png", ts, sanitizeFileName(keyword), sanitizeFileName(period), sanitizeFileName(suffix))
	fullPath := filepath.Join(dir, name)
	if err := os.WriteFile(fullPath, decoded, 0o644); err != nil {
		return "", "", 0, err
	}

	assetURL := "/assets/baidu-index/" + name
	return name, assetURL, len(decoded), nil
}

func buildBaiduTrendURL(keyword string) string {
	k := strings.TrimSpace(keyword)
	esc := url.QueryEscape(k)
	return fmt.Sprintf("https://index.baidu.com/v2/main/index.html#/trend/%s?words=%s", esc, esc)
}

func sanitizeFileName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	raw = strings.ReplaceAll(raw, " ", "-")
	re := regexp.MustCompile(`[\\/:*?"<>|]+`)
	clean := re.ReplaceAllString(raw, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "unknown"
	}
	return clean
}

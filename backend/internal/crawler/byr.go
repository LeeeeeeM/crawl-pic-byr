package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"crawl-pic/backend/internal/models"
	"crawl-pic/backend/internal/repository"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

type BYRService struct {
	repo *repository.Repository
}

type boardArticleRef struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type articlePayload struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Images  []string `json:"images"`
}

type devToolsVersion struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func NewBYR(repo *repository.Repository) *BYRService {
	return &BYRService{repo: repo}
}

func (s *BYRService) Run(parent context.Context, jobID uuid.UUID, cfg models.BYRCrawlConfig) error {
	if err := s.repo.ValidateBYRConfig(cfg); err != nil {
		return err
	}

	wsURL, err := fetchWebSocketDebuggerURL(cfg.RemoteDebugURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 2*time.Hour)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	if err := s.ensureBoardReachable(browserCtx, cfg.BoardName, cfg.StartPage); err != nil {
		return err
	}

	seenArticles := make(map[string]struct{})
	maxPage := cfg.StartPage + cfg.MaxPages - 1
	for page := cfg.StartPage; page <= maxPage; page++ {
		boardURL := fmt.Sprintf("https://bbs.byr.cn/#!board/%s?p=%d", cfg.BoardName, page)
		refs, err := s.extractBoardArticles(browserCtx, boardURL)
		if err != nil {
			return fmt.Errorf("extract board page %d failed: %w", page, err)
		}
		if len(refs) == 0 {
			break
		}

		newOnPage := 0
		for _, ref := range refs {
			if _, ok := seenArticles[ref.URL]; ok {
				continue
			}
			seenArticles[ref.URL] = struct{}{}
			newOnPage++

			detail, postURL, err := s.extractArticleDetail(browserCtx, ref.URL)
			if err != nil {
				continue
			}
			if len(detail.Images) == 0 {
				continue
			}

			title := strings.TrimSpace(detail.Title)
			if title == "" {
				title = strings.TrimSpace(ref.Title)
			}
			if title == "" {
				title = postURL
			}

			qctx, qcancel := repository.WithTimeout(ctx)
			postID, err := s.repo.UpsertPost(qctx, jobID, postURL, title, strings.TrimSpace(detail.Content))
			qcancel()
			if err != nil {
				continue
			}

			for _, rawImg := range detail.Images {
				imgURL := resolveBYRURL(postURL, rawImg)
				if imgURL == "" {
					continue
				}
				keep, _ := shouldKeepImageBySize(ctx, imgURL, cfg.MinImageBytes)
				if !keep {
					continue
				}
				ictx, icancel := repository.WithTimeout(ctx)
				_ = s.repo.InsertPhoto(ictx, jobID, postID, imgURL, "")
				icancel()
			}
		}

		if newOnPage == 0 {
			break
		}
	}

	return nil
}

func (s *BYRService) ensureBoardReachable(ctx context.Context, boardName string, page int) error {
	boardURL := fmt.Sprintf("https://bbs.byr.cn/#!board/%s?p=%d", boardName, page)
	if _, err := s.extractBoardArticles(ctx, boardURL); err != nil {
		return err
	}

	var isNotLoggedIn bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`(() => (document.body && document.body.innerText.includes('您未登录')))()`, &isNotLoggedIn),
	)
	if err != nil {
		return err
	}
	if isNotLoggedIn {
		return fmt.Errorf("BYR is not logged in. Please login in Chrome first")
	}
	return nil
}

func (s *BYRService) extractBoardArticles(ctx context.Context, boardURL string) ([]boardArticleRef, error) {
	var refs []boardArticleRef
	err := chromedp.Run(ctx,
		chromedp.Navigate(boardURL),
		chromedp.Sleep(1800*time.Millisecond),
		chromedp.Evaluate(`(() => {
			const anchors = Array.from(document.querySelectorAll('#body .board-list tbody tr .title_9 a:first-child'));
			return anchors
				.map(a => ({ title: (a.textContent || '').trim(), url: a.href || '' }))
				.filter(item => item.url);
		})()`, &refs),
	)
	if err != nil {
		return nil, err
	}
	return refs, nil
}

func (s *BYRService) extractArticleDetail(ctx context.Context, articleURL string) (articlePayload, string, error) {
	urlToVisit := normalizeArticleURL(articleURL)
	var payload articlePayload
	var currentURL string
	err := chromedp.Run(ctx,
		chromedp.Navigate(urlToVisit),
		chromedp.Sleep(1800*time.Millisecond),
		chromedp.Location(&currentURL),
		chromedp.Evaluate(`(() => {
			const wrap = document.querySelector('#body .a-wrap');
			if (!wrap) {
				return { title: '', content: '', images: [] };
			}
			const titleEl = wrap.querySelector('.a-title') || document.querySelector('#notice_nav a:last-child') || document.querySelector('title');
			const contentEl = wrap.querySelector('.a-content') || wrap;
			const images = Array.from(contentEl.querySelectorAll('img'))
				.map(img => img.getAttribute('src') || img.getAttribute('data-src') || img.getAttribute('data-original') || '')
				.filter(Boolean);
			return {
				title: titleEl ? (titleEl.textContent || '').trim() : '',
				content: (contentEl.innerText || '').trim(),
				images
			};
		})()`, &payload),
	)
	if err != nil {
		return articlePayload{}, "", err
	}

	if currentURL == "" {
		currentURL = urlToVisit
	}
	return payload, currentURL, nil
}

func fetchWebSocketDebuggerURL(remoteDebugURL string) (string, error) {
	base := strings.TrimRight(remoteDebugURL, "/")
	resp, err := http.Get(base + "/json/version")
	if err != nil {
		return "", fmt.Errorf("connect chrome devtools failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("chrome devtools returned status %d", resp.StatusCode)
	}

	var version devToolsVersion
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", fmt.Errorf("parse chrome devtools version failed: %w", err)
	}
	if version.WebSocketDebuggerURL == "" {
		return "", fmt.Errorf("webSocketDebuggerUrl is empty, ensure Chrome started with --remote-debugging-port")
	}

	return version.WebSocketDebuggerURL, nil
}

func normalizeArticleURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "#!") {
		return "https://bbs.byr.cn/" + raw
	}
	if strings.HasPrefix(raw, "/#!") {
		return "https://bbs.byr.cn" + raw
	}
	if strings.HasPrefix(raw, "/") {
		return "https://bbs.byr.cn" + raw
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://bbs.byr.cn/#!" + raw
}

func resolveBYRURL(baseURL, refURL string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(strings.TrimSpace(refURL))
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

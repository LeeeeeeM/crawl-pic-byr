package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"crawl-pic/backend/internal/models"
	"crawl-pic/backend/internal/repository"

	"github.com/chromedp/chromedp"
	"github.com/google/uuid"
)

type CDPService struct {
	repo *repository.Repository
}

type cdpPagePayload struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Images  []string `json:"images"`
}

func NewCDP(repo *repository.Repository) *CDPService {
	return &CDPService{repo: repo}
}

func (s *CDPService) Run(parent context.Context, jobID uuid.UUID, cfg models.CDPCrawlConfig) error {
	if err := s.repo.ValidateCDPConfig(cfg); err != nil {
		return err
	}

	wsURL, err := fetchWebSocketDebuggerURL(cfg.RemoteDebugURL)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 45*time.Minute)
	defer cancel()

	allocCtx, allocCancel := chromedp.NewRemoteAllocator(ctx, wsURL)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	if cfg.WaitAfterLoadMs <= 0 {
		cfg.WaitAfterLoadMs = 1800
	}
	if cfg.ImageSelector == "" {
		cfg.ImageSelector = "img"
	}
	if cfg.TitleSelector == "" {
		cfg.TitleSelector = "title"
	}
	if cfg.ContentSelector == "" {
		cfg.ContentSelector = "body"
	}

	currentURL, payload, err := s.extractPage(browserCtx, cfg)
	if err != nil {
		return err
	}
	if len(payload.Images) == 0 {
		return nil
	}

	title := strings.TrimSpace(payload.Title)
	if title == "" {
		title = currentURL
	}

	qctx, qcancel := repository.WithTimeout(ctx)
	postID, err := s.repo.UpsertPost(qctx, jobID, currentURL, title, strings.TrimSpace(payload.Content))
	qcancel()
	if err != nil {
		return err
	}

	seen := make(map[string]struct{})
	for _, raw := range payload.Images {
		fullURL := resolveURL(currentURL, strings.TrimSpace(raw))
		if fullURL == "" {
			continue
		}
		if _, ok := seen[fullURL]; ok {
			continue
		}
		seen[fullURL] = struct{}{}

		keep, _ := shouldKeepImageBySize(ctx, fullURL, cfg.MinImageBytes)
		if !keep {
			continue
		}

		ictx, icancel := repository.WithTimeout(ctx)
		_ = s.repo.InsertPhoto(ictx, jobID, postID, fullURL, "")
		icancel()
	}

	return nil
}

func (s *CDPService) extractPage(ctx context.Context, cfg models.CDPCrawlConfig) (string, cdpPagePayload, error) {
	startURL := strings.TrimSpace(cfg.StartURL)
	if startURL == "" {
		return "", cdpPagePayload{}, fmt.Errorf("startURL is required")
	}

	parsed, err := url.Parse(startURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", cdpPagePayload{}, fmt.Errorf("invalid startURL: %s", startURL)
	}

	var currentURL string
	var payload cdpPagePayload
	actions := []chromedp.Action{
		chromedp.Navigate(startURL),
	}
	if strings.TrimSpace(cfg.PageReadySelector) != "" {
		actions = append(actions, chromedp.WaitReady(cfg.PageReadySelector, chromedp.ByQuery))
	}
	evalJS := fmt.Sprintf(`(() => {
		const contentSelector = %q;
		const titleSelector = %q;
		const imageSelector = %q;
		const root = document.querySelector(contentSelector) || document.body;
		const titleEl = document.querySelector(titleSelector) || document.querySelector('title');
		const images = root
			? Array.from(root.querySelectorAll(imageSelector)).map((img) =>
				img.currentSrc || img.getAttribute('src') || img.getAttribute('data-src') || img.getAttribute('data-original') || ''
			)
			: [];
		return {
			title: titleEl ? (titleEl.textContent || '').trim() : '',
			content: root ? (root.innerText || '').trim() : '',
			images: images.filter(Boolean),
		};
	})()`, cfg.ContentSelector, cfg.TitleSelector, cfg.ImageSelector)

	actions = append(actions,
		chromedp.Sleep(time.Duration(cfg.WaitAfterLoadMs)*time.Millisecond),
		chromedp.Location(&currentURL),
		chromedp.Evaluate(evalJS, &payload),
	)

	if err := chromedp.Run(ctx, actions...); err != nil {
		return "", cdpPagePayload{}, err
	}
	if currentURL == "" {
		currentURL = startURL
	}
	return currentURL, payload, nil
}

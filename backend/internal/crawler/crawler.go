package crawler

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"crawl-pic/backend/internal/models"
	"crawl-pic/backend/internal/repository"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/google/uuid"
)

type Service struct {
	repo *repository.Repository
}

func New(repo *repository.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Run(parent context.Context, jobID uuid.UUID, cfg models.CrawlConfig) error {
	if err := s.repo.ValidateConfig(cfg); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(parent, 45*time.Minute)
	defer cancel()

	if cfg.MaxListPages <= 0 {
		cfg.MaxListPages = 10
	}
	if cfg.MaxPosts <= 0 {
		cfg.MaxPosts = 200
	}
	if cfg.RequestTimeoutSecs <= 0 {
		cfg.RequestTimeoutSecs = 20
	}
	if cfg.ImageSelector == "" {
		cfg.ImageSelector = "img"
	}
	if cfg.PostTitleSelector == "" {
		cfg.PostTitleSelector = "title"
	}

	listCollector := colly.NewCollector(
		colly.AllowedDomains(cfg.AllowedDomains...),
	)
	postCollector := listCollector.Clone()

	listCollector.SetRequestTimeout(time.Duration(cfg.RequestTimeoutSecs) * time.Second)
	postCollector.SetRequestTimeout(time.Duration(cfg.RequestTimeoutSecs) * time.Second)

	var (
		mu              sync.Mutex
		listPagesCount  int
		postsCount      int
		visitedPostURLs = make(map[string]struct{})
	)

	listCollector.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})

	postCollector.OnRequest(func(r *colly.Request) {
		select {
		case <-ctx.Done():
			r.Abort()
		default:
		}
	})

	listCollector.OnHTML(cfg.PostLinkSelector, func(e *colly.HTMLElement) {
		postURL := e.Request.AbsoluteURL(strings.TrimSpace(e.Attr("href")))
		if postURL == "" {
			return
		}

		mu.Lock()
		if postsCount >= cfg.MaxPosts {
			mu.Unlock()
			return
		}
		if _, ok := visitedPostURLs[postURL]; ok {
			mu.Unlock()
			return
		}
		visitedPostURLs[postURL] = struct{}{}
		postsCount++
		mu.Unlock()

		_ = postCollector.Visit(postURL)
	})

	if cfg.NextPageSelector != "" {
		listCollector.OnHTML(cfg.NextPageSelector, func(e *colly.HTMLElement) {
			nextURL := e.Request.AbsoluteURL(strings.TrimSpace(e.Attr("href")))
			if nextURL == "" {
				return
			}

			mu.Lock()
			if listPagesCount >= cfg.MaxListPages {
				mu.Unlock()
				return
			}
			listPagesCount++
			mu.Unlock()

			_ = listCollector.Visit(nextURL)
		})
	}

	postCollector.OnResponse(func(r *colly.Response) {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(r.Body)))
		if err != nil {
			return
		}

		title := strings.TrimSpace(doc.Find(cfg.PostTitleSelector).First().Text())
		if title == "" {
			title = r.Request.URL.String()
		}
		content := strings.TrimSpace(doc.Text())

		qctx, qcancel := repository.WithTimeout(ctx)
		postID, err := s.repo.UpsertPost(qctx, jobID, r.Request.URL.String(), title, content)
		qcancel()
		if err != nil {
			return
		}

		doc.Find(cfg.ImageSelector).Each(func(_ int, sel *goquery.Selection) {
			src := strings.TrimSpace(attrWithFallback(sel, "src", "data-src", "data-original"))
			if src == "" {
				return
			}

			fullURL := resolveURL(r.Request.URL.String(), src)
			if fullURL == "" {
				return
			}
			keep, _ := shouldKeepImageBySize(ctx, fullURL, cfg.MinImageBytes)
			if !keep {
				return
			}

			altText := strings.TrimSpace(attrWithFallback(sel, "alt"))

			ictx, icancel := repository.WithTimeout(ctx)
			_ = s.repo.InsertPhoto(ictx, jobID, postID, fullURL, altText)
			icancel()
		})
	})

	var collectorErr error
	listCollector.OnError(func(_ *colly.Response, err error) {
		collectorErr = err
	})
	postCollector.OnError(func(_ *colly.Response, err error) {
		collectorErr = err
	})

	for _, startURL := range cfg.StartURLs {
		if err := listCollector.Visit(startURL); err != nil {
			return err
		}
	}

	listCollector.Wait()
	postCollector.Wait()

	if ctx.Err() != nil {
		return fmt.Errorf("crawl timed out: %w", ctx.Err())
	}
	if collectorErr != nil {
		return collectorErr
	}

	return nil
}

func attrWithFallback(sel *goquery.Selection, attrs ...string) string {
	for _, key := range attrs {
		if v, ok := sel.Attr(key); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func resolveURL(baseURL, raw string) string {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	parsedRef, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsedBase.ResolveReference(parsedRef).String()
}

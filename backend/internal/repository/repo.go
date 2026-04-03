package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strings"
	"time"

	"crawl-pic/backend/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) CreateJob(ctx context.Context, cfg models.CrawlConfig) (models.CrawlJob, error) {
	return r.CreateRawJob(ctx, cfg.SiteName, cfg)
}

func (r *Repository) CreateRawJob(ctx context.Context, siteName string, payload any) (models.CrawlJob, error) {
	jobID := uuid.New()
	configBytes, err := json.Marshal(payload)
	if err != nil {
		return models.CrawlJob{}, err
	}

	var job models.CrawlJob
	err = r.pool.QueryRow(ctx, `
		INSERT INTO crawl_jobs (id, site_name, status, config)
		VALUES ($1, $2, 'pending', $3)
		RETURNING id, site_name, status, error_message, started_at, finished_at, created_at
	`, jobID, siteName, configBytes).Scan(
		&job.ID,
		&job.SiteName,
		&job.Status,
		&job.Error,
		&job.StartedAt,
		&job.FinishedAt,
		&job.CreatedAt,
	)
	if err != nil {
		return models.CrawlJob{}, err
	}

	return job, nil
}

func (r *Repository) MarkJobRunning(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE crawl_jobs
		SET status = 'running', started_at = NOW(), error_message = NULL
		WHERE id = $1
	`, jobID)
	return err
}

func (r *Repository) MarkJobFailed(ctx context.Context, jobID uuid.UUID, crawlErr error) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE crawl_jobs
		SET status = 'failed', error_message = $2, finished_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, jobID, crawlErr.Error())
	return err
}

func (r *Repository) MarkJobFinished(ctx context.Context, jobID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE crawl_jobs
		SET status = 'done', finished_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, jobID)
	return err
}

func (r *Repository) MarkJobCancelled(ctx context.Context, jobID uuid.UUID, reason string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "cancelled by user"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE crawl_jobs
		SET status = 'failed', error_message = $2, finished_at = NOW()
		WHERE id = $1 AND status = 'running'
	`, jobID, reason)
	return err
}

func (r *Repository) GetJob(ctx context.Context, jobID uuid.UUID) (models.CrawlJob, error) {
	var job models.CrawlJob
	err := r.pool.QueryRow(ctx, `
		SELECT id, site_name, status, error_message, started_at, finished_at, created_at
		FROM crawl_jobs
		WHERE id = $1
	`, jobID).Scan(
		&job.ID,
		&job.SiteName,
		&job.Status,
		&job.Error,
		&job.StartedAt,
		&job.FinishedAt,
		&job.CreatedAt,
	)
	return job, err
}

func (r *Repository) ListJobs(ctx context.Context, limit int) ([]models.CrawlJob, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, site_name, status, error_message, started_at, finished_at, created_at
		FROM crawl_jobs
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]models.CrawlJob, 0)
	for rows.Next() {
		var job models.CrawlJob
		if scanErr := rows.Scan(
			&job.ID,
			&job.SiteName,
			&job.Status,
			&job.Error,
			&job.StartedAt,
			&job.FinishedAt,
			&job.CreatedAt,
		); scanErr != nil {
			return nil, scanErr
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

func (r *Repository) DeleteJob(ctx context.Context, jobID uuid.UUID) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM crawl_jobs
		WHERE id = $1
	`, jobID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *Repository) UpsertPost(ctx context.Context, jobID uuid.UUID, postURL, title, content string) (int64, error) {
	var postID int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO posts (job_id, title, content, url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (job_id, url)
		DO UPDATE SET title = EXCLUDED.title, content = EXCLUDED.content
		RETURNING id
	`, jobID, title, content, postURL).Scan(&postID)
	return postID, err
}

func (r *Repository) InsertPhoto(ctx context.Context, jobID uuid.UUID, postID int64, photoURL, altText string) error {
	var fileName *string
	if parsed, err := url.Parse(photoURL); err == nil {
		base := path.Base(parsed.Path)
		if base != "." && base != "/" && base != "" {
			fileName = &base
		}
	}

	var alt *string
	if altText != "" {
		alt = &altText
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO photos (job_id, post_id, url, file_name, alt_text)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (job_id, url)
		DO NOTHING
	`, jobID, postID, photoURL, fileName, alt)
	return err
}

func (r *Repository) ListPosts(ctx context.Context, jobID uuid.UUID) ([]models.Post, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, job_id, title, content, url, created_at
		FROM posts
		WHERE job_id = $1
		ORDER BY id ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var p models.Post
		if scanErr := rows.Scan(&p.ID, &p.JobID, &p.Title, &p.Content, &p.URL, &p.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		posts = append(posts, p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return posts, nil
}

func (r *Repository) ListPhotos(ctx context.Context, jobID uuid.UUID) ([]models.Photo, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, job_id, post_id, url, file_name, alt_text, created_at
		FROM photos
		WHERE job_id = $1
		ORDER BY id ASC
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := make([]models.Photo, 0)
	for rows.Next() {
		var p models.Photo
		if scanErr := rows.Scan(&p.ID, &p.JobID, &p.PostID, &p.URL, &p.FileName, &p.AltText, &p.CreatedAt); scanErr != nil {
			return nil, scanErr
		}
		photos = append(photos, p)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return photos, nil
}

func (r *Repository) ValidateConfig(cfg models.CrawlConfig) error {
	if cfg.SiteName == "" {
		return errors.New("siteName is required")
	}
	if len(cfg.StartURLs) == 0 {
		return errors.New("startUrls must not be empty")
	}
	if cfg.PostLinkSelector == "" {
		return errors.New("postLinkSelector is required")
	}
	for _, u := range cfg.StartURLs {
		parsed, err := url.Parse(u)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("invalid start URL: %s", u)
		}
	}
	if cfg.MaxListPages <= 0 {
		cfg.MaxListPages = 10
	}
	if cfg.MaxPosts <= 0 {
		cfg.MaxPosts = 200
	}
	if cfg.MinImageBytes < 0 {
		return errors.New("minImageBytes must be >= 0")
	}
	return nil
}

func (r *Repository) ValidateBYRConfig(cfg models.BYRCrawlConfig) error {
	if cfg.BoardName == "" {
		return errors.New("boardName is required")
	}
	if cfg.StartPage <= 0 {
		return errors.New("startPage must be >= 1")
	}
	if cfg.MaxPages <= 0 {
		return errors.New("maxPages must be >= 1")
	}
	if cfg.RemoteDebugURL == "" {
		return errors.New("remoteDebugUrl is required, e.g. http://127.0.0.1:9222")
	}
	if _, err := url.Parse(cfg.RemoteDebugURL); err != nil {
		return fmt.Errorf("invalid remoteDebugUrl: %w", err)
	}
	if cfg.MinImageBytes < 0 {
		return errors.New("minImageBytes must be >= 0")
	}
	return nil
}

func WithTimeout(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, 30*time.Second)
}

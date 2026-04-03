package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"sync"

	"crawl-pic/backend/internal/crawler"
	"crawl-pic/backend/internal/models"
	"crawl-pic/backend/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	repo       *repository.Repository
	crawler    *crawler.Service
	byrCrawler *crawler.BYRService
	mu         sync.Mutex
	cancels    map[uuid.UUID]context.CancelFunc
}

func New(repo *repository.Repository, crawlerSvc *crawler.Service, byrSvc *crawler.BYRService) *Handler {
	return &Handler{
		repo:       repo,
		crawler:    crawlerSvc,
		byrCrawler: byrSvc,
		cancels:    make(map[uuid.UUID]context.CancelFunc),
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/health", h.health)
	r.Post("/api/jobs", h.createJob)
	r.Get("/api/jobs", h.listJobs)
	r.Post("/api/byr/jobs", h.createBYRJob)
	r.Post("/api/jobs/{id}/cancel", h.cancelJob)
	r.Delete("/api/jobs/{id}", h.deleteJob)
	r.Get("/api/jobs/{id}", h.getJob)
	r.Get("/api/jobs/{id}/posts", h.listPosts)
	r.Get("/api/jobs/{id}/photos", h.listPhotos)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) createJob(w http.ResponseWriter, r *http.Request) {
	var cfg models.CrawlConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.repo.ValidateConfig(cfg); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.repo.CreateJob(r.Context(), cfg)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go h.runJob(job.ID, cfg)

	jsonResponse(w, http.StatusAccepted, job)
}

func (h *Handler) createBYRJob(w http.ResponseWriter, r *http.Request) {
	var cfg models.BYRCrawlConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if cfg.SiteName == "" {
		cfg.SiteName = "byr-" + cfg.BoardName
	}
	if cfg.StartPage <= 0 {
		cfg.StartPage = 1
	}
	if cfg.MaxPages <= 0 {
		cfg.MaxPages = 2000
	}
	if cfg.RemoteDebugURL == "" {
		cfg.RemoteDebugURL = "http://127.0.0.1:9222"
	}

	if err := h.repo.ValidateBYRConfig(cfg); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.repo.CreateRawJob(r.Context(), cfg.SiteName, cfg)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go h.runBYRJob(job.ID, cfg)

	jsonResponse(w, http.StatusAccepted, job)
}

func (h *Handler) runJob(jobID uuid.UUID, cfg models.CrawlConfig) {
	runCtx, runCancel := context.WithCancel(context.Background())
	h.setCancel(jobID, runCancel)
	defer h.clearCancel(jobID)
	defer runCancel()

	ctx, cancel := repository.WithTimeout(rBackground())
	defer cancel()
	_ = h.repo.MarkJobRunning(ctx, jobID)

	if err := h.crawler.Run(runCtx, jobID, cfg); err != nil {
		fctx, fcancel := repository.WithTimeout(rBackground())
		_ = h.repo.MarkJobFailed(fctx, jobID, err)
		fcancel()
		return
	}

	dctx, dcancel := repository.WithTimeout(rBackground())
	_ = h.repo.MarkJobFinished(dctx, jobID)
	dcancel()
}

func (h *Handler) runBYRJob(jobID uuid.UUID, cfg models.BYRCrawlConfig) {
	runCtx, runCancel := context.WithCancel(context.Background())
	h.setCancel(jobID, runCancel)
	defer h.clearCancel(jobID)
	defer runCancel()

	ctx, cancel := repository.WithTimeout(rBackground())
	defer cancel()
	_ = h.repo.MarkJobRunning(ctx, jobID)

	if err := h.byrCrawler.Run(runCtx, jobID, cfg); err != nil {
		fctx, fcancel := repository.WithTimeout(rBackground())
		_ = h.repo.MarkJobFailed(fctx, jobID, err)
		fcancel()
		return
	}

	dctx, dcancel := repository.WithTimeout(rBackground())
	_ = h.repo.MarkJobFinished(dctx, jobID)
	dcancel()
}

func (h *Handler) getJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.repo.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "job not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, job)
}

func (h *Handler) cancelJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.repo.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "job not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if job.Status != "running" {
		jsonResponse(w, http.StatusOK, map[string]string{
			"id":     jobID.String(),
			"status": "already_stopped",
		})
		return
	}

	cancel, found := h.getCancel(jobID)
	if !found {
		ctx, done := repository.WithTimeout(rBackground())
		defer done()
		_ = h.repo.MarkJobCancelled(ctx, jobID, "cancelled by user (state-only)")
		jsonResponse(w, http.StatusOK, map[string]string{
			"id":     jobID.String(),
			"status": "cancelled_state_only",
		})
		return
	}

	cancel()

	ctx, done := repository.WithTimeout(rBackground())
	defer done()
	_ = h.repo.MarkJobCancelled(ctx, jobID, "cancelled by user")

	jsonResponse(w, http.StatusOK, map[string]string{
		"id":     jobID.String(),
		"status": "cancelling",
	})
}

func (h *Handler) deleteJob(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.repo.GetJob(r.Context(), jobID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonError(w, http.StatusNotFound, "job not found")
			return
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if job.Status == "running" {
		cancel, found := h.getCancel(jobID)
		if !found {
			jsonError(w, http.StatusConflict, "job is running in another server process; stop it first")
			return
		}
		cancel()
	}

	ctx, done := repository.WithTimeout(rBackground())
	defer done()
	deleted, err := h.repo.DeleteJob(ctx, jobID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !deleted {
		jsonError(w, http.StatusNotFound, "job not found")
		return
	}

	h.clearCancel(jobID)
	jsonResponse(w, http.StatusOK, map[string]string{
		"id":     jobID.String(),
		"status": "deleted",
	})
}

func (h *Handler) listJobs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			jsonError(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = parsed
	}

	jobs, err := h.repo.ListJobs(r.Context(), limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, jobs)
}

func (h *Handler) listPosts(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseJobID(w, r)
	if !ok {
		return
	}

	posts, err := h.repo.ListPosts(r.Context(), jobID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, posts)
}

func (h *Handler) listPhotos(w http.ResponseWriter, r *http.Request) {
	jobID, ok := parseJobID(w, r)
	if !ok {
		return
	}

	photos, err := h.repo.ListPhotos(r.Context(), jobID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, photos)
}

func parseJobID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id := chi.URLParam(r, "id")
	jobID, err := uuid.Parse(id)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid UUID")
		return uuid.Nil, false
	}
	return jobID, true
}

func jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	jsonResponse(w, status, map[string]string{"error": msg})
}

func (h *Handler) setCancel(jobID uuid.UUID, cancel context.CancelFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cancels[jobID] = cancel
}

func (h *Handler) clearCancel(jobID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.cancels, jobID)
}

func (h *Handler) getCancel(jobID uuid.UUID) (context.CancelFunc, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	cancel, ok := h.cancels[jobID]
	return cancel, ok
}

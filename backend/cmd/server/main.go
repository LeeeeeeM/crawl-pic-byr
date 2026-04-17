package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crawl-pic/backend/internal/config"
	"crawl-pic/backend/internal/crawler"
	"crawl-pic/backend/internal/db"
	"crawl-pic/backend/internal/handlers"
	"crawl-pic/backend/internal/repository"
	"crawl-pic/backend/migrations"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := migrations.Apply(ctx, pool); err != nil {
		log.Fatal(err)
	}

	repo := repository.New(pool)
	crawlerSvc := crawler.New(repo)
	byrSvc := crawler.NewBYR(repo)
	cdpSvc := crawler.NewCDP(repo)
	baiduIndexSvc := crawler.NewBaiduIndex(repo, cfg.AssetsDir)
	h := handlers.New(repo, crawlerSvc, byrSvc, cdpSvc, baiduIndexSvc)

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	h.RegisterRoutes(r)
	if err := os.MkdirAll(cfg.AssetsDir, 0o755); err != nil {
		log.Fatal(err)
	}
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(cfg.AssetsDir))))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("backend listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

// Command tabitha runs the web server and its supporting CLI subcommands
// (migrate, jobs, promote-admin) from a single binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jhash/tabitha/internal/config"
	"github.com/jhash/tabitha/internal/db"
	"github.com/jhash/tabitha/internal/jobs"
	"github.com/jhash/tabitha/internal/web"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "serve":
		return serve(cfg)
	case "migrate":
		return runMigrate(cfg, args[1:])
	case "jobs":
		return runJobs(cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q (want: serve, migrate, jobs)", cmd)
	}
}

func runMigrate(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tabitha migrate [up|down]")
	}
	switch args[0] {
	case "up":
		if err := db.MigrateUp(cfg.DatabaseURL); err != nil {
			return err
		}
		return withPool(cfg, func(ctx context.Context, pool *pgxpool.Pool) error {
			return jobs.MigrateUp(ctx, pool)
		})
	case "down":
		return db.MigrateDown(cfg.DatabaseURL)
	default:
		return fmt.Errorf("unknown migrate subcommand %q (want: up, down)", args[0])
	}
}

func runJobs(cfg config.Config, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: tabitha jobs [enqueue toc-sync|work]")
	}
	switch args[0] {
	case "enqueue":
		if len(args) < 2 || args[1] != "toc-sync" {
			return fmt.Errorf("usage: tabitha jobs enqueue [toc-sync]")
		}
		return withPool(cfg, func(ctx context.Context, pool *pgxpool.Pool) error {
			client, err := jobs.NewClient(pool, db.New(pool))
			if err != nil {
				return err
			}
			if err := jobs.EnqueueTocSync(ctx, client); err != nil {
				return err
			}
			log.Println("tabitha: enqueued toc_sync job")
			return nil
		})
	case "work":
		// Dev/ops convenience: process whatever's currently queued, then
		// exit, without standing up the full HTTP server. The running
		// server (once built) keeps its own River client processing
		// continuously in the background for the periodic/production case.
		return withPool(cfg, func(ctx context.Context, pool *pgxpool.Pool) error {
			client, err := jobs.NewClient(pool, db.New(pool))
			if err != nil {
				return err
			}
			if err := client.Start(ctx); err != nil {
				return fmt.Errorf("starting river client: %w", err)
			}
			log.Println("tabitha: working queued jobs for up to 30s...")
			time.Sleep(30 * time.Second)
			return client.Stop(ctx)
		})
	default:
		return fmt.Errorf("unknown jobs subcommand %q (want: enqueue, work)", args[0])
	}
}

func withPool(cfg config.Config, fn func(context.Context, *pgxpool.Pool) error) error {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()
	return fn(ctx, pool)
}

func serve(cfg config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	queries := db.New(pool)

	jobClient, err := jobs.NewClient(pool, queries)
	if err != nil {
		return fmt.Errorf("creating job client: %w", err)
	}
	if err := jobClient.Start(ctx); err != nil {
		return fmt.Errorf("starting job client: %w", err)
	}

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: web.NewRouter(cfg, queries),
	}

	go func() {
		<-ctx.Done()
		log.Println("tabitha: shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
		_ = jobClient.Stop(shutdownCtx)
	}()

	log.Printf("tabitha: listening on %s", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving http: %w", err)
	}
	return nil
}

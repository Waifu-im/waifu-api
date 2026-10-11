// Command server runs the waifu-api HTTP service.
//
// # Startup order
//
// Configuration is loaded and validated first, then the database connection is
// established, and only then does the listener start. A failure at any step
// exits non-zero with a message on stderr.
//
// This ordering is deliberate. The C# service fails fast on missing
// configuration (Program.cs ValidateConfiguration), and this service keeps that
// behaviour. Starting with a half-understood configuration would mean a process
// that accepts traffic it cannot correctly authorise.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"waifuapi/internal/auth"
	"waifuapi/internal/config"
	"waifuapi/internal/httpapi"
	"waifuapi/internal/logging"
	"waifuapi/internal/repo"
	"waifuapi/internal/storage"
	"waifuapi/internal/store"
)

func main() {
	if err := run(); err != nil {
		// Written to stderr directly rather than through the logger, because the
		// logger may not have been constructed yet.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// run contains the startup and shutdown sequence, kept separate from main so it
// can return an error rather than calling os.Exit, which defers would otherwise
// skip.
func run() error {
	// ---- Configuration -------------------------------------------------
	cfg, err := config.Load(config.NewLoader())
	if err != nil {
		return err
	}

	log := logging.New(logging.Options{
		Environment: cfg.Environment,
		Level:       os.Getenv("LOG_LEVEL"),
	})
	slog.SetDefault(log)

	log.Info("starting waifu-api",
		"environment", cfg.Environment,
		"port", cfg.Port,
		"base_path", cfg.BasePathOrRoot(),
	)

	// ---- Database ------------------------------------------------------
	// A generous startup timeout: a cold database or a busy pooler can take a
	// moment, and failing immediately would turn a transient condition into a
	// crash loop.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	db, err := store.Open(startupCtx, store.Config{
		URL:            cfg.DatabaseURL,
		MaxConnections: 8,
	})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

	log.Info("database connected")

	// ---- HTTP ----------------------------------------------------------
	data := repo.New(db.Pool())
	tokens := auth.NewTokenIssuer(cfg.JWT)
	authenticator := auth.NewAuthenticator(tokens)

	// Object storage is required for uploads. It is constructed eagerly so a
	// misconfigured bucket fails at startup rather than on the first upload,
	// matching the behaviour of the C# service.
	files, err := storage.NewS3Store(storage.Config{
		Provider:       storage.Provider(cfg.S3.Provider),
		AccessKey:      cfg.S3.AccessKey,
		SecretKey:      cfg.S3.SecretKey,
		ServiceURL:     cfg.S3.ServiceURL,
		Region:         cfg.S3.Region,
		BucketName:     cfg.S3.BucketName,
		ForcePathStyle: cfg.S3.ForcePathStyle,
	})
	if err != nil {
		return err
	}
	log.Info("object storage configured",
		"provider", cfg.S3.Provider,
		"bucket", cfg.S3.BucketName,
		"pathStyle", cfg.S3.ForcePathStyle)

	server := httpapi.NewServer(cfg, db, data, log, files)

	mux := http.NewServeMux()
	server.Register(mux)

	handler := server.Handler(mux, authenticator, data)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: handler,

		// Generous timeouts. They exist to shed genuinely stuck connections,
		// not to cut off slow-but-progressing work.
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// ---- Serve ---------------------------------------------------------
	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
			return
		}
		serveErr <- nil
	}()

	// ---- Shutdown ------------------------------------------------------
	// Wait for SIGINT or SIGTERM. Render, Docker and every other host send
	// SIGTERM on deploy, so handling it is what makes zero-downtime restarts
	// possible.
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		if err != nil {
			return fmt.Errorf("http server: %w", err)
		}
		return nil

	case sig := <-shutdownSignal:
		log.Info("shutdown signal received", "signal", sig.String())

		// In-flight requests get a bounded window to finish, so a deploy does
		// not sever requests that are already being served.
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancelShutdown()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("graceful shutdown failed", "error", err)
			return fmt.Errorf("shutdown: %w", err)
		}

		log.Info("shutdown complete")
		return nil
	}
}

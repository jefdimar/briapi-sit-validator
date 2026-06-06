package main

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/gdrive"
	"github.com/jefdimar/briapi-sit-validator/internal/metrics"
)

const version = "2.0.0"

func main() {
	// Load .env from the first available candidate path so the server works
	// regardless of whether it is started via `go run ./cmd/server` (cwd is
	// the module root), a built binary in bin/, or any other working directory.
	config.LoadFirstDotEnv()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/rules.yaml"
	}

	mgr, err := config.NewManager(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Start dynamic config reload watcher in the background
	go watchConfig(mgr)

	cfg := mgr.Get()
	features := config.LoadFeatures()

	// Google Drive client is created only when credentials are present AND
	// the DRIVE_ENABLED feature flag is not explicitly set to false.
	var driveClient *gdrive.Client
	if features.DriveEnabled {
		dc, driveConfigured := gdrive.NewClientFromEnv()
		if driveConfigured {
			slog.Info("google drive integration enabled")
			driveClient = dc
		}
	} else {
		slog.Info("google drive integration disabled via DRIVE_ENABLED flag")
	}

	router := setupRouter(mgr, driveClient)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Server.Port, "version", version)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
	slog.Info("server stopped")
}

// watchConfig polls the config file for changes and reloads it.
func watchConfig(mgr *config.Manager) {
	var lastMod time.Time
	if info, err := os.Stat(mgr.Path()); err == nil {
		lastMod = info.ModTime()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		info, err := os.Stat(mgr.Path())
		if err != nil {
			continue
		}
		if info.ModTime().After(lastMod) {
			slog.Info("config file changed, reloading config...", "path", mgr.Path())
			if err := mgr.Reload(); err != nil {
				slog.Error("failed to reload config", "error", err)
			} else {
				slog.Info("config reloaded successfully")
				lastMod = info.ModTime()
			}
		}
	}
}

// requestLogger is a Gin middleware that generates or propagates an
// X-Request-ID and logs each request with timing information.
// The request ID is also returned in the response header so clients
// can correlate logs.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%016x", rand.Uint64())
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
		status := c.Writer.Status()
		latency := time.Since(start)
		slog.Info("request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", status,
			"latency_ms", latency.Milliseconds(),
		)

		// Record Prometheus HTTP metrics
		statusStr := fmt.Sprintf("%d", status)
		metrics.HttpRequestsTotal.WithLabelValues(c.Request.Method, c.Request.URL.Path, statusStr).Inc()
		metrics.HttpRequestDurationSeconds.WithLabelValues(c.Request.Method, c.Request.URL.Path, statusStr).Observe(latency.Seconds())
	}
}

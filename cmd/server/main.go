package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/gdrive"
)

const version = "1.0.0"

// loadDotEnv reads a .env file and sets any key=value pairs as environment
// variables, skipping blank lines and comments.  Existing OS env vars always
// take precedence so that CI/CD secrets are never overridden.
// dotEnvCandidates returns a prioritised list of paths to look for a .env file.
// This covers: running from the project root, running a binary from bin/, and
// running via `go run ./cmd/server` where the binary lands in a temp dir.
func dotEnvCandidates() []string {
	candidates := []string{".env"} // cwd — works for `go run` from project root

	// Walk up from the executable's location (covers bin/sit-validator → root).
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for range 4 { // up to 4 levels up
			candidates = append(candidates, filepath.Join(dir, ".env"))
			dir = filepath.Dir(dir)
		}
	}
	return candidates
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional; missing file is not an error
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		// Strip surrounding single or double quotes.
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if os.Getenv(key) == "" { // don't override pre-existing env vars
			os.Setenv(key, val)
		}
	}
}

func main() {
	// Try loading .env from several candidate locations so the server works
	// regardless of whether it is started via `go run ./cmd/server` (cwd is
	// the module root), a built binary in bin/, or any other working directory.
	for _, p := range dotEnvCandidates() {
		if _, err := os.Stat(p); err == nil {
			loadDotEnv(p)
			break
		}
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/rules.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	driveClient, driveConfigured := gdrive.NewClientFromEnv()
	if driveConfigured {
		slog.Info("google drive integration enabled")
	}

	router := setupRouter(cfg, driveClient)

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

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%016x", rand.Uint64())
		}
		c.Set("request_id", requestID)
		c.Next()
		slog.Info("request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

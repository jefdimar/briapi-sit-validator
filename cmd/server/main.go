package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/jefdimar/briapi-sit-validator/internal/reporter"
	"github.com/jefdimar/briapi-sit-validator/internal/validator"
)

const version = "1.0.0"

func main() {
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

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	maxBytes := int64(cfg.Server.MaxUploadSizeMB) << 20

	router.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	})

	router.POST("/api/v1/validate", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		fh, err := c.FormFile("file")
		if err != nil {
			if strings.Contains(err.Error(), "http: request body too large") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": fmt.Sprintf("file too large, max %dMB", cfg.Server.MaxUploadSizeMB)})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
			return
		}

		if strings.ToLower(filepath.Ext(fh.Filename)) != ".xlsx" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file format: expected .xlsx"})
			return
		}

		p, err := parser.Open(fh)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": fmt.Sprintf("cannot parse excel file: %s", err.Error())})
			return
		}
		defer p.Close()

		// Resolve optional sheet filter from query param.
		var filterSheets []string
		if raw := c.Query("sheets"); raw != "" {
			for _, s := range strings.Split(raw, ",") {
				if t := strings.TrimSpace(s); t != "" {
					filterSheets = append(filterSheets, t)
				}
			}
		}

		report := validator.Validate(p, cfg, filterSheets)

		if len(report.Sheets) == 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no recognizable product sheets found"})
			return
		}

		format := strings.ToLower(c.DefaultQuery("format", "json"))
		if format == "excel" {
			data, err := reporter.BuildExcel(p, report, cfg)
			if err != nil {
				slog.Error("excel reporter error", "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			c.Header("Content-Disposition", `attachment; filename="sit_validation_report.xlsx"`)
			c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
			return
		}

		c.JSON(http.StatusOK, reporter.BuildJSON(report))
	})

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
			requestID = fmt.Sprintf("%d", start.UnixNano())
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

package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/apierror"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/gdrive"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/jefdimar/briapi-sit-validator/internal/reporter"
	"github.com/jefdimar/briapi-sit-validator/internal/validator"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// setupRouter builds and returns the Gin engine with all routes registered.
// Extracted from main() to allow handler-level testing.
// driveClient is optional; pass nil (or omit) when Drive integration is not configured.
func setupRouter(mgr *config.Manager, driveClients ...*gdrive.Client) *gin.Engine {
	var driveClient *gdrive.Client
	if len(driveClients) > 0 {
		driveClient = driveClients[0]
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	// validFormats contains the accepted values for the format query parameter.
	validFormats := map[string]bool{"json": true, "excel": true}

	// healthHandler is reused across API versions.
	healthHandler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	}

	// sheetsHandler is reused across API versions.
	sheetsHandler := func(c *gin.Context) {
		cfg := mgr.Get()
		maxBytes := int64(cfg.Server.MaxUploadSizeMB) << 20
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		requestID, _ := c.Get("request_id")
		reqID := fmt.Sprintf("%v", requestID)

		fh, err := c.FormFile("file")
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				slog.Debug("file too large", "request_id", reqID, "error", err)
				apierror.TooLarge(c, fmt.Sprintf("file too large, max %dMB", cfg.Server.MaxUploadSizeMB))
				return
			}
			slog.Debug("file upload parameter missing", "request_id", reqID, "error", err)
			apierror.BadRequest(c, "file is required")
			return
		}

		if strings.ToLower(filepath.Ext(fh.Filename)) != ".xlsx" {
			slog.Debug("invalid file extension", "request_id", reqID, "filename", fh.Filename)
			apierror.BadRequest(c, "invalid file format: expected .xlsx")
			return
		}

		p, err := parser.Open(fh)
		if err != nil {
			slog.Debug("failed to parse excel file", "request_id", reqID, "error", err)
			apierror.UnprocessableEntity(c, fmt.Sprintf("cannot parse excel file: %s", err.Error()))
			return
		}
		defer p.Close()

		skipSet := makeSkipSet(cfg.Excel.SkipSheets)
		var productSheets []string
		for _, s := range p.SheetNames() {
			if !skipSet[s] {
				productSheets = append(productSheets, s)
			}
		}
		if productSheets == nil {
			productSheets = []string{}
		}

		c.JSON(http.StatusOK, gin.H{"sheets": productSheets})
	}

	// validateHandler is reused across API versions.
	validateHandler := func(c *gin.Context) {
		cfg := mgr.Get()
		maxBytes := int64(cfg.Server.MaxUploadSizeMB) << 20
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		requestID, _ := c.Get("request_id")
		reqID := fmt.Sprintf("%v", requestID)

		// Input validation: reject unknown format values.
		format := strings.ToLower(c.DefaultQuery("format", "json"))
		if !validFormats[format] {
			slog.Debug("invalid format query parameter", "request_id", reqID, "format", format)
			apierror.BadRequest(c, fmt.Sprintf("invalid format: %q, must be json or excel", format))
			return
		}

		fh, err := c.FormFile("file")
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				slog.Debug("file too large", "request_id", reqID, "error", err)
				apierror.TooLarge(c, fmt.Sprintf("file too large, max %dMB", cfg.Server.MaxUploadSizeMB))
				return
			}
			slog.Debug("file upload parameter missing", "request_id", reqID, "error", err)
			apierror.BadRequest(c, "file is required")
			return
		}

		if strings.ToLower(filepath.Ext(fh.Filename)) != ".xlsx" {
			slog.Debug("invalid file extension", "request_id", reqID, "filename", fh.Filename)
			apierror.BadRequest(c, "invalid file format: expected .xlsx")
			return
		}

		p, err := parser.Open(fh)
		if err != nil {
			slog.Debug("failed to parse excel file", "request_id", reqID, "error", err)
			apierror.UnprocessableEntity(c, fmt.Sprintf("cannot parse excel file: %s", err.Error()))
			return
		}
		defer p.Close()

		// sheets can be supplied as a form field (body) or query string.
		// Form field takes precedence; query string is the fallback.
		sheetsRaw := c.PostForm("sheets")
		if sheetsRaw == "" {
			sheetsRaw = c.Query("sheets")
		}
		var filterSheets []string
		for _, s := range strings.Split(sheetsRaw, ",") {
			if t := strings.TrimSpace(s); t != "" {
				filterSheets = append(filterSheets, t)
			}
		}

		report := validator.Validate(p, cfg, filterSheets, reqID)

		if len(report.Sheets) == 0 {
			slog.Debug("no product sheets found after filtering", "request_id", reqID, "filter", filterSheets)
			apierror.UnprocessableEntity(c, "no recognizable product sheets found")
			return
		}

		if format == "excel" {
			data, err := reporter.BuildExcel(p, report, cfg)
			if err != nil {
				slog.Error("excel reporter error", "request_id", reqID, "error", err)
				apierror.Internal(c, "internal server error")
				return
			}
			if driveClient != nil {
				driveURL, driveErr := driveClient.UploadExcel(c.Request.Context(), fh.Filename, data)
				if driveErr != nil {
					slog.Error("drive upload error", "request_id", reqID, "error", driveErr)
				} else {
					c.Header("X-Drive-File-URL", driveURL)
				}
			}
			c.Header("Content-Disposition", `attachment; filename="sit_validation_report.xlsx"`)
			c.Data(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", data)
			return
		}

		c.JSON(http.StatusOK, reporter.BuildJSON(report))
	}

	// Register metrics route
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Register v1 routes (existing API).
	v1 := router.Group("/api/v1")
	{
		v1.GET("/health", healthHandler)
		v1.POST("/sheets", sheetsHandler)
		v1.POST("/validate", validateHandler)
	}

	// Register v2 routes (same handlers — versioned prefix for future divergence).
	v2 := router.Group("/api/v2")
	{
		v2.GET("/health", healthHandler)
		v2.POST("/sheets", sheetsHandler)
		v2.POST("/validate", validateHandler)
	}

	return router
}

func makeSkipSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

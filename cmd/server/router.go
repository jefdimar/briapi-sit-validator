package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jefdimar/briapi-sit-validator/internal/config"
	"github.com/jefdimar/briapi-sit-validator/internal/gdrive"
	"github.com/jefdimar/briapi-sit-validator/internal/parser"
	"github.com/jefdimar/briapi-sit-validator/internal/reporter"
	"github.com/jefdimar/briapi-sit-validator/internal/validator"
)

// setupRouter builds and returns the Gin engine with all routes registered.
// Extracted from main() to allow handler-level testing.
// driveClient is optional; pass nil (or omit) when Drive integration is not configured.
func setupRouter(cfg *config.Config, driveClients ...*gdrive.Client) *gin.Engine {
	var driveClient *gdrive.Client
	if len(driveClients) > 0 {
		driveClient = driveClients[0]
	}
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(requestLogger())

	maxBytes := int64(cfg.Server.MaxUploadSizeMB) << 20

	router.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": version})
	})

	router.POST("/api/v1/sheets", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		fh, err := c.FormFile("file")
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
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
	})

	router.POST("/api/v1/validate", func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

		fh, err := c.FormFile("file")
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
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

		requestID, _ := c.Get("request_id")
		reqID := fmt.Sprintf("%v", requestID)

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
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no recognizable product sheets found"})
			return
		}

		format := strings.ToLower(c.DefaultQuery("format", "json"))
		if format == "excel" {
			data, err := reporter.BuildExcel(p, report, cfg)
			if err != nil {
				slog.Error("excel reporter error", "request_id", reqID, "error", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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
	})

	return router
}

func makeSkipSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

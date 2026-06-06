// Package apierror provides a standardized error response format for all API
// handlers. Every error returned to clients uses the same JSON shape:
//
//	{"error": "<message>", "code": <http_status>}
package apierror

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the canonical error body returned by every handler.
type Response struct {
	Error string `json:"error"`
	Code  int    `json:"code"`
}

// JSON writes a standardised error response and aborts the request chain.
func JSON(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, Response{
		Error: msg,
		Code:  status,
	})
}

// Convenience helpers for common status codes.

// BadRequest responds with 400.
func BadRequest(c *gin.Context, msg string) {
	JSON(c, http.StatusBadRequest, msg)
}

// NotFound responds with 404.
func NotFound(c *gin.Context, msg string) {
	JSON(c, http.StatusNotFound, msg)
}

// UnprocessableEntity responds with 422.
func UnprocessableEntity(c *gin.Context, msg string) {
	JSON(c, http.StatusUnprocessableEntity, msg)
}

// TooLarge responds with 413.
func TooLarge(c *gin.Context, msg string) {
	JSON(c, http.StatusRequestEntityTooLarge, msg)
}

// Internal responds with 500.
func Internal(c *gin.Context, msg string) {
	JSON(c, http.StatusInternalServerError, msg)
}

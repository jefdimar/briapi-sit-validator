package apierror

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func performWithHandler(handler gin.HandlerFunc) *httptest.ResponseRecorder {
	router := gin.New()
	router.GET("/test", handler)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)
	return w
}

func TestJSON_WritesStandardisedBody(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		JSON(c, http.StatusTeapot, "I'm a teapot")
	})

	assert.Equal(t, http.StatusTeapot, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "I'm a teapot", resp.Error)
	assert.Equal(t, http.StatusTeapot, resp.Code)
}

func TestBadRequest(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		BadRequest(c, "bad input")
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestNotFound(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		NotFound(c, "not here")
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestUnprocessableEntity(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		UnprocessableEntity(c, "bad entity")
	})
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTooLarge(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		TooLarge(c, "too big")
	})
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestInternal(t *testing.T) {
	w := performWithHandler(func(c *gin.Context) {
		Internal(c, "oops")
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestJSON_AbortsChain(t *testing.T) {
	router := gin.New()
	secondHandlerCalled := false
	router.GET("/test",
		func(c *gin.Context) {
			JSON(c, http.StatusForbidden, "denied")
		},
		func(c *gin.Context) {
			secondHandlerCalled = true
		},
	)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, secondHandlerCalled, "subsequent handler should not run after Abort")
}

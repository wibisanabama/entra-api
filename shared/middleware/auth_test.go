package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"entra-api/shared/middleware"
	"github.com/gin-gonic/gin"
)

func TestRequireInternalSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	secret := "super-secret-internal-key-999"
	_ = os.Setenv("INTERNAL_SERVICE_SECRET", secret)
	defer os.Unsetenv("INTERNAL_SERVICE_SECRET")

	r := gin.New()
	r.POST("/api/v1/internal/test", middleware.RequireInternalSecret(secret), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	t.Run("Rejects request with missing secret header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/test", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("Rejects request with invalid secret header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/test", nil)
		req.Header.Set("X-Internal-Secret", "wrong-secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 Forbidden, got %d", w.Code)
		}
	})

	t.Run("Accepts request with valid secret header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/test", nil)
		req.Header.Set("X-Internal-Secret", secret)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK, got %d", w.Code)
		}
	})
}

func TestCORS_PreflightAndHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CORS())
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	t.Run("Handles OPTIONS preflight with 204 No Content", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodOptions, "/ping", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("Access-Control-Request-Method", "GET")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204 No Content for OPTIONS, got %d", w.Code)
		}

		if allowOrigin := w.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "http://localhost:3000" {
			t.Errorf("expected Access-Control-Allow-Origin http://localhost:3000, got %s", allowOrigin)
		}
	})
}

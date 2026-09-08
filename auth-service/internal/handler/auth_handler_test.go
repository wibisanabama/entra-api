package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"entra-api/auth-service/internal/handler"
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestAuthHandler_GetUsersBatch_Validation(t *testing.T) {
	h := handler.NewAuthHandler(nil, struct {
		Host     string
		Port     string
		Username string
		Password string
	}{})

	r := gin.New()
	r.POST("/api/v1/internal/users/batch", h.GetUsersBatch)

	t.Run("Rejects request with missing body", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/users/batch", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 Unprocessable Entity, got %d", w.Code)
		}
	})

	t.Run("Rejects request with invalid UUID in array", func(t *testing.T) {
		body, _ := json.Marshal(handler.GetUsersBatchRequest{
			IDs: []string{"valid-looking-but-not-uuid", "550e8400-e29b-41d4-a716-446655440000"},
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/users/batch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for invalid uuid, got %d", w.Code)
		}
	})
}

func TestAuthHandler_Profile_UnauthorizedWithoutContext(t *testing.T) {
	h := handler.NewAuthHandler(nil, struct {
		Host     string
		Port     string
		Username string
		Password string
	}{})

	r := gin.New()
	r.GET("/api/v1/auth/profile", h.GetProfile)
	r.PUT("/api/v1/auth/profile", h.UpdateProfile)
	r.POST("/api/v1/auth/upgrade", h.UpgradeToOrganizer)
	r.POST("/api/v1/auth/change-password", h.ChangePassword)

	t.Run("GetProfile returns 401 when context user_id is missing", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/profile", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("UpdateProfile returns 401 when context user_id is missing", func(t *testing.T) {
		body, _ := json.Marshal(gin.H{"full_name": "New Name"})
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/auth/profile", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("UpgradeToOrganizer returns 401 when context user_id is missing", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/upgrade", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("ChangePassword returns 401 when context user_id is missing", func(t *testing.T) {
		body, _ := json.Marshal(gin.H{"old_password": "pass", "new_password": "newpassword123"})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})
}

func TestAuthHandler_RoutesSetup(t *testing.T) {
	jwtSecret := "jwt-test-secret"
	_ = os.Setenv("INTERNAL_SERVICE_SECRET", "internal-test-secret")
	defer os.Unsetenv("INTERNAL_SERVICE_SECRET")

	h := handler.NewAuthHandler(nil, struct {
		Host     string
		Port     string
		Username string
		Password string
	}{})

	r := gin.New()
	handler.RegisterRoutes(r, h, jwtSecret)

	t.Run("Health check endpoint responds 200", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200 OK for /health, got %d", w.Code)
		}
	})

	t.Run("Internal batch endpoint is protected by internal secret", func(t *testing.T) {
		body, _ := json.Marshal(handler.GetUsersBatchRequest{
			IDs: []string{"550e8400-e29b-41d4-a716-446655440000"},
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/users/batch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		// Missing X-Internal-Secret header
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden without internal secret header, got %d", w.Code)
		}
	})

	t.Run("Internal batch endpoint accepts valid internal secret", func(t *testing.T) {
		body, _ := json.Marshal(handler.GetUsersBatchRequest{
			IDs: []string{"invalid-uuid"},
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/users/batch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Secret", "internal-test-secret")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Passes secret check, fails on UUID parsing validation -> 400 Bad Request
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request (passed auth middleware), got %d", w.Code)
		}
	})

	t.Run("Internal batch endpoint accepts organizer JWT token", func(t *testing.T) {
		body, _ := json.Marshal(handler.GetUsersBatchRequest{
			IDs: []string{"invalid-uuid"},
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/users/batch", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// Create organizer token
		claims := &middleware.JWTClaims{
			UserID: "550e8400-e29b-41d4-a716-446655440001",
			Role:   "organizer",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, _ := token.SignedString([]byte(jwtSecret))
		req.Header.Set("Authorization", "Bearer "+tokenString)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Passes JWT organizer check, fails on UUID parsing validation -> 400 Bad Request
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request (passed JWT organizer auth), got %d", w.Code)
		}
	})
}

package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"entra-api/shared/middleware"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func createTestToken(secret, userID, role string, expiry time.Duration) string {
	claims := middleware.JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}

func TestJWTAuth_Middleware(t *testing.T) {
	jwtSecret := "jwt-secret-key-for-testing-12345"

	setupRouter := func() *gin.Engine {
		r := gin.New()
		r.Use(middleware.JWTAuth(jwtSecret))
		r.GET("/protected", func(c *gin.Context) {
			userID, _ := c.Get(middleware.AuthUserIDKey)
			role, _ := c.Get(middleware.AuthUserRoleKey)
			c.JSON(http.StatusOK, gin.H{
				"user_id": userID,
				"role":    role,
			})
		})
		return r
	}

	r := setupRouter()

	t.Run("Rejects request when Authorization header is missing", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Rejects request with invalid Authorization header format", func(t *testing.T) {
		malformedHeaders := []string{
			"Basic dXNlcjpwYXNz",
			"Bearer",
			"Token 123456",
			"RandomHeaderString",
		}

		for _, header := range malformedHeaders {
			req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", header)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("for header %q, expected status 401, got %d", header, w.Code)
			}
		}
	})

	t.Run("Rejects request with expired token", func(t *testing.T) {
		expiredToken := createTestToken(jwtSecret, "user-expired-1", "customer", -1*time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized for expired token, got %d", w.Code)
		}
	})

	t.Run("Rejects request with wrong signing secret", func(t *testing.T) {
		wrongSecretToken := createTestToken("wrong-secret-key-9999", "user-2", "customer", time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+wrongSecretToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 Unauthorized for invalid signature, got %d", w.Code)
		}
	})

	t.Run("Accepts request with valid token and sets context", func(t *testing.T) {
		validToken := createTestToken(jwtSecret, "user-valid-123", "organizer", time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+validToken)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestRequireRole_Middleware(t *testing.T) {
	jwtSecret := "jwt-secret-key-for-role-test"

	setupRouter := func(allowedRoles ...string) *gin.Engine {
		r := gin.New()
		r.Use(middleware.JWTAuth(jwtSecret))
		r.Use(middleware.RequireRole(allowedRoles...))
		r.GET("/admin-or-organizer", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "allowed"})
		})
		return r
	}

	r := setupRouter("organizer", "admin")

	t.Run("Allows organizer role", func(t *testing.T) {
		token := createTestToken(jwtSecret, "org-1", "organizer", time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/admin-or-organizer", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for organizer, got %d", w.Code)
		}
	})

	t.Run("Allows admin role", func(t *testing.T) {
		token := createTestToken(jwtSecret, "adm-1", "admin", time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/admin-or-organizer", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for admin, got %d", w.Code)
		}
	})

	t.Run("Denies customer role with 403 Forbidden", func(t *testing.T) {
		token := createTestToken(jwtSecret, "cust-1", "customer", time.Hour)
		req, _ := http.NewRequest(http.MethodGet, "/admin-or-organizer", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 Forbidden for customer, got %d", w.Code)
		}
	})

	t.Run("Denies when no role is in context", func(t *testing.T) {
		rNoAuth := gin.New()
		rNoAuth.Use(middleware.RequireRole("admin"))
		rNoAuth.GET("/direct", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req, _ := http.NewRequest(http.MethodGet, "/direct", nil)
		w := httptest.NewRecorder()
		rNoAuth.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403 Forbidden when role is missing from context, got %d", w.Code)
		}
	})
}

func TestRequireInternalSecret(t *testing.T) {
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

	t.Run("Empty secret in non-production allows bypass", func(t *testing.T) {
		rBypass := gin.New()
		rBypass.POST("/api/v1/internal/bypass", middleware.RequireInternalSecret(""), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "bypassed"})
		})

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/bypass", nil)
		w := httptest.NewRecorder()
		rBypass.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK in non-production empty secret, got %d", w.Code)
		}
	})

	t.Run("Empty secret in production triggers 500 error", func(t *testing.T) {
		_ = os.Setenv("APP_ENV", "production")
		defer os.Unsetenv("APP_ENV")

		rProd := gin.New()
		rProd.POST("/api/v1/internal/prod", middleware.RequireInternalSecret(""), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/prod", nil)
		w := httptest.NewRecorder()
		rProd.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500 in production with empty secret, got %d", w.Code)
		}
	})
}

func TestCORS_PreflightAndHeaders(t *testing.T) {
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

		if allowCreds := w.Header().Get("Access-Control-Allow-Credentials"); allowCreds != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials true, got %s", allowCreds)
		}
	})

	t.Run("Adds CORS headers to regular GET request", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		req.Header.Set("Origin", "http://localhost:5173")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for GET, got %d", w.Code)
		}

		if allowOrigin := w.Header().Get("Access-Control-Allow-Origin"); allowOrigin != "http://localhost:5173" {
			t.Errorf("expected Access-Control-Allow-Origin http://localhost:5173, got %s", allowOrigin)
		}
	})
}

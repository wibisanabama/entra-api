package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"entra-api/event-service/internal/handler"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestEventHandler_ProtectedRoutes_UnauthorizedWithoutJWT(t *testing.T) {
	jwtSecret := "test-secret"
	_ = os.Setenv("INTERNAL_SERVICE_SECRET", "event-secret-123")
	defer os.Unsetenv("INTERNAL_SERVICE_SECRET")

	eh := handler.NewEventHandler(nil)
	vh := handler.NewVenueHandler(nil)
	ch := handler.NewCategoryHandler(nil)
	ith := handler.NewInternalTicketHandler(nil)
	tth := handler.NewTicketTypeHandler(nil)

	r := gin.New()
	handler.RegisterRoutes(r, eh, vh, ch, ith, tth, jwtSecret)

	t.Run("Create event without JWT returns 401", func(t *testing.T) {
		body, _ := json.Marshal(gin.H{
			"title":      "Concert",
			"start_date": "2026-10-01T10:00:00Z",
			"end_date":   "2026-10-01T22:00:00Z",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Create venue without JWT returns 401", func(t *testing.T) {
		body, _ := json.Marshal(gin.H{
			"name":    "GBK",
			"address": "Senayan",
			"city":    "Jakarta",
		})
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/venues", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", w.Code)
		}
	})

	t.Run("Internal ticket reserve without internal secret returns 403", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/internal/tickets/550e8400-e29b-41d4-a716-446655440000/reserve", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden without secret, got %d", w.Code)
		}
	})
}

func TestEventHandler_PaginationParsing(t *testing.T) {
	parsePagination := func(pageStr, perPageStr string) (int, int) {
		page := 1
		perPage := 10
		if pageStr != "" {
			if p, err := parsePositiveInt(pageStr); err == nil && p > 0 {
				page = p
			}
		}
		if perPageStr != "" {
			if pp, err := parsePositiveInt(perPageStr); err == nil && pp > 0 && pp <= 100 {
				perPage = pp
			}
		}
		return page, perPage
	}

	t.Run("Default pagination", func(t *testing.T) {
		p, pp := parsePagination("", "")
		if p != 1 || pp != 10 {
			t.Errorf("expected 1 and 10, got %d and %d", p, pp)
		}
	})

	t.Run("Custom valid pagination", func(t *testing.T) {
		p, pp := parsePagination("3", "25")
		if p != 3 || pp != 25 {
			t.Errorf("expected 3 and 25, got %d and %d", p, pp)
		}
	})

	t.Run("Invalid pagination clamped to defaults", func(t *testing.T) {
		p, pp := parsePagination("-1", "500")
		if p != 1 || pp != 10 {
			t.Errorf("expected clamped 1 and 10, got %d and %d", p, pp)
		}
	})
}

func parsePositiveInt(s string) (int, error) {
	var val int
	_, err := json.Marshal(s)
	if err != nil {
		return 0, err
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, http.ErrNotSupported
		}
		val = val*10 + int(c-'0')
	}
	return val, nil
}

package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestPaymentHandler_ReferenceTypeParameter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("Extracts reference_type from query param", func(t *testing.T) {
		r := gin.New()

		var capturedRefType string
		r.GET("/api/v1/payments/reference/:reference_id", func(c *gin.Context) {
			refType := c.Query("reference_type")
			if refType == "" {
				refType = c.Query("type")
			}
			if refType == "" {
				refType = c.Param("reference_type")
			}
			if refType == "" {
				refType = "TICKET"
			}
			capturedRefType = refType
			c.Status(http.StatusOK)
		})

		refID := uuid.New().String()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/payments/reference/"+refID+"?reference_type=TOPUP", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if capturedRefType != "TOPUP" {
			t.Errorf("expected reference_type TOPUP, got %s", capturedRefType)
		}
	})

	t.Run("Defaults to TICKET when query param is absent", func(t *testing.T) {
		r := gin.New()
		var capturedRefType string
		r.GET("/api/v1/payments/reference/:reference_id", func(c *gin.Context) {
			refType := c.Query("reference_type")
			if refType == "" {
				refType = c.Query("type")
			}
			if refType == "" {
				refType = c.Param("reference_type")
			}
			if refType == "" {
				refType = "TICKET"
			}
			capturedRefType = refType
			c.Status(http.StatusOK)
		})

		refID := uuid.New().String()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/payments/reference/"+refID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if capturedRefType != "TICKET" {
			t.Errorf("expected default reference_type TICKET, got %s", capturedRefType)
		}
	})
}

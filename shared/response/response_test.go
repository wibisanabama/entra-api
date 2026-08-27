package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"entra-api/shared/response"
	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewMeta(t *testing.T) {
	tests := []struct {
		name               string
		page               int
		perPage            int
		total              int64
		expectedTotalPages int
	}{
		{
			name:               "Zero items has 0 total pages",
			page:               1,
			perPage:            10,
			total:              0,
			expectedTotalPages: 0,
		},
		{
			name:               "Exact multiple has exact pages",
			page:               1,
			perPage:            10,
			total:              30,
			expectedTotalPages: 3,
		},
		{
			name:               "Remainder rounds up to next page",
			page:               1,
			perPage:            10,
			total:              31,
			expectedTotalPages: 4,
		},
		{
			name:               "Single item has 1 page",
			page:               1,
			perPage:            20,
			total:              1,
			expectedTotalPages: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := response.NewMeta(tt.page, tt.perPage, tt.total)
			if meta.Page != tt.page {
				t.Errorf("expected page %d, got %d", tt.page, meta.Page)
			}
			if meta.PerPage != tt.perPage {
				t.Errorf("expected per_page %d, got %d", tt.perPage, meta.PerPage)
			}
			if meta.Total != tt.total {
				t.Errorf("expected total %d, got %d", tt.total, meta.Total)
			}
			if meta.TotalPages != tt.expectedTotalPages {
				t.Errorf("expected total_pages %d, got %d", tt.expectedTotalPages, meta.TotalPages)
			}
		})
	}
}

func TestResponseHelpers(t *testing.T) {
	t.Run("Success response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		response.Success(c, http.StatusOK, "data retrieved", gin.H{"key": "value"})

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var body response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if !body.Success {
			t.Errorf("expected Success true, got false")
		}
		if body.Message != "data retrieved" {
			t.Errorf("expected message 'data retrieved', got %s", body.Message)
		}
	})

	t.Run("SuccessWithPagination response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		meta := response.NewMeta(1, 10, 25)
		data := []string{"item1", "item2"}
		response.SuccessWithPagination(c, "paginated list", data, meta)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var body response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if !body.Success || body.Meta == nil || body.Meta.TotalPages != 3 {
			t.Errorf("unexpected body structure: %+v", body)
		}
	})

	t.Run("Error response with extra error details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		errDetails := []string{"field email is required"}
		response.Error(c, http.StatusBadRequest, "bad request", errDetails)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var body response.Response
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if body.Success {
			t.Errorf("expected Success false, got true")
		}
		if body.Message != "bad request" {
			t.Errorf("expected message 'bad request', got %s", body.Message)
		}
		if body.Errors == nil {
			t.Errorf("expected Errors not nil")
		}
	})

	t.Run("ValidationError response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		errMap := gin.H{"email": "invalid email format"}
		response.ValidationError(c, errMap)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected status 422, got %d", w.Code)
		}
	})

	t.Run("NotFound response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		response.NotFound(c, "resource not found")

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status 404, got %d", w.Code)
		}
	})

	t.Run("Unauthorized response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		response.Unauthorized(c, "unauthorized access")

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	t.Run("Forbidden response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		response.Forbidden(c, "forbidden action")

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	t.Run("InternalError response", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		response.InternalError(c, "internal server error occurred")

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})
}

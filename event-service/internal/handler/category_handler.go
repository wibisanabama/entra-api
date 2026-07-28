package handler

import (
	"net/http"

	"entra-api/event-service/internal/repository/db"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
)

type CategoryHandler struct {
	queries *db.Queries
}

func NewCategoryHandler(queries *db.Queries) *CategoryHandler {
	return &CategoryHandler{queries: queries}
}

func (h *CategoryHandler) List(c *gin.Context) {
	categories, err := h.queries.ListCategories(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to list categories")
		return
	}

	response.Success(c, http.StatusOK, "categories retrieved", categories)
}

package handler

import (
	"net/http"
	"entra-api/shared/middleware"
	"entra-api/shared/response"
	"entra-api/ticket-service/internal/service"

	"github.com/gin-gonic/gin"
)

type OrderHandler struct {
	ticketService *service.TicketService
}

func NewOrderHandler(ticketService *service.TicketService) *OrderHandler {
	return &OrderHandler{ticketService: ticketService}
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	userID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	order, err := h.ticketService.CreateOrder(c.Request.Context(), userID.(string), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "order created", order)
}

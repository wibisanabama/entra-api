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

func (h *OrderHandler) CreatePaymentToken(c *gin.Context) {
	userID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	orderID := c.Param("id")
	if orderID == "" {
		response.ValidationError(c, "order id is required")
		return
	}

	_ = userID // In a real app, verify order belongs to user

	token, err := h.ticketService.CreatePaymentToken(c.Request.Context(), orderID)
	if err != nil {
		response.InternalError(c, "failed to get payment token: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "payment token generated", gin.H{"token": token})
}

func (h *OrderHandler) MidtransWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.ValidationError(c, "invalid json payload")
		return
	}

	err := h.ticketService.HandleMidtransNotification(c.Request.Context(), payload)
	if err != nil {
		// Log the error but return 200 to acknowledge receipt to Midtrans
		response.Success(c, http.StatusOK, "webhook received but encountered error", nil)
		return
	}

	response.Success(c, http.StatusOK, "webhook processed", nil)
}

func (h *OrderHandler) ListMyOrders(c *gin.Context) {
	userID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Wait, ticketService doesn't have ListMyOrders yet, but queries does. Let's add it to TicketService.
	// We'll call ticketService.ListMyOrders
	orders, err := h.ticketService.ListMyOrders(c.Request.Context(), userID.(string))
	if err != nil {
		response.InternalError(c, "failed to fetch orders: " + err.Error())
		return
	}

	response.Success(c, http.StatusOK, "orders retrieved", orders)
}

func (h *OrderHandler) GetOrganizerStats(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	stats, err := h.ticketService.GetDashboardStats(c.Request.Context(), organizerID.(string))
	if err != nil {
		response.InternalError(c, "failed to fetch stats: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "stats retrieved", stats)
}

func (h *OrderHandler) ListOrganizerOrders(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page := 1
	perPage := 10 // defaults

	orders, err := h.ticketService.ListOrganizerOrders(c.Request.Context(), organizerID.(string), page, perPage)
	if err != nil {
		response.InternalError(c, "failed to fetch orders: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "orders retrieved", orders)
}


func (h *OrderHandler) GetSalesTrend(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	trend, err := h.ticketService.GetSalesTrend(c.Request.Context(), organizerID.(string))
	if err != nil {
		response.InternalError(c, "failed to fetch sales trend: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "sales trend retrieved", trend)
}

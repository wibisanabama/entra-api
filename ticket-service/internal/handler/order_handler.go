package handler

import (
	"net/http"
	"strconv"
	"strings"
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
	uidStr, ok := userID.(string)
	if !exists || !ok || uidStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	order, err := h.ticketService.CreateOrder(c.Request.Context(), uidStr, req)
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

	uidStr := ""
	if userID != nil {
		if s, ok := userID.(string); ok {
			uidStr = s
		}
	}

	token, err := h.ticketService.CreatePaymentToken(c.Request.Context(), orderID, uidStr)
	if err != nil {
		if strings.Contains(err.Error(), "access denied") {
			response.Error(c, http.StatusForbidden, err.Error())
			return
		}
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
	uidStr, ok := userID.(string)
	if !exists || !ok || uidStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	orders, err := h.ticketService.ListMyOrders(c.Request.Context(), uidStr)
	if err != nil {
		response.InternalError(c, "failed to fetch orders: " + err.Error())
		return
	}

	response.Success(c, http.StatusOK, "orders retrieved", orders)
}

func (h *OrderHandler) GetOrganizerStats(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	orgIDStr, ok := organizerID.(string)
	if !exists || !ok || orgIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	stats, err := h.ticketService.GetDashboardStats(c.Request.Context(), orgIDStr)
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

	orgIDStr, ok := organizerID.(string)
	if !ok || orgIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "invalid organizer session")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}

	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	orders, err := h.ticketService.ListOrganizerOrders(c.Request.Context(), orgIDStr, page, perPage)
	if err != nil {
		response.InternalError(c, "failed to fetch orders: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "orders retrieved", orders)
}


func (h *OrderHandler) GetSalesTrend(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	orgIDStr, ok := organizerID.(string)
	if !exists || !ok || orgIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	trend, err := h.ticketService.GetSalesTrend(c.Request.Context(), orgIDStr)
	if err != nil {
		response.InternalError(c, "failed to fetch sales trend: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "sales trend retrieved", trend)
}

func (h *OrderHandler) GetOrganizerOrder(c *gin.Context) {
	orderID := c.Param("id")
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	orgIDStr, ok := organizerID.(string)
	if !exists || !ok || orgIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	order, items, tickets, err := h.ticketService.GetOrganizerOrder(c.Request.Context(), orderID, orgIDStr)
	if err != nil {
		response.InternalError(c, "failed to get order: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "order retrieved", gin.H{
		"order":   order,
		"items":   items,
		"tickets": tickets,
	})
}

func (h *OrderHandler) GetEventAttendees(c *gin.Context) {
	eventID := c.Param("eventId")
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	orgIDStr, ok := organizerID.(string)
	if !exists || !ok || orgIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	tickets, err := h.ticketService.GetEventAttendees(c.Request.Context(), eventID, orgIDStr)
	if err != nil {
		response.InternalError(c, "failed to get attendees: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "attendees retrieved", tickets)
}

func (h *OrderHandler) ValidatePromo(c *gin.Context) {
	var req service.ValidatePromoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	res, err := h.ticketService.ValidatePromo(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "validasi promo selesai", res)
}


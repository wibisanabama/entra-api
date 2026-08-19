package handler

import (
	"net/http"
	"entra-api/shared/middleware"
	"entra-api/shared/response"
	"entra-api/ticket-service/internal/service"

	"github.com/gin-gonic/gin"
)

type TicketHandler struct {
	ticketService *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{ticketService: ticketService}
}

func (h *TicketHandler) ListMyTickets(c *gin.Context) {
	userID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	tickets, err := h.ticketService.ListMyTickets(c.Request.Context(), userID.(string))
	if err != nil {
		response.InternalError(c, "failed to fetch tickets: " + err.Error())
		return
	}

	response.Success(c, http.StatusOK, "tickets retrieved", tickets)
}

func (h *TicketHandler) GetTicketByCodeInternal(c *gin.Context) {
	ticketCode := c.Param("code")
	ticket, err := h.ticketService.GetTicketByCode(c.Request.Context(), ticketCode)
	if err != nil {
		response.Error(c, http.StatusNotFound, "ticket not found: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "ticket retrieved", ticket)
}

func (h *TicketHandler) GetEventGateStatsInternal(c *gin.Context) {
	eventID := c.Param("eventId")
	stats, err := h.ticketService.GetEventGateStats(c.Request.Context(), eventID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "failed to get event gate stats: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "event gate stats retrieved", stats)
}

func (h *TicketHandler) TransferTicket(c *gin.Context) {
	userID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	ticketID := c.Param("id")
	if ticketID == "" {
		response.Error(c, http.StatusBadRequest, "ticket id required")
		return
	}

	var req service.TransferTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	res, err := h.ticketService.TransferTicket(c.Request.Context(), userID.(string), ticketID, req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "tiket berhasil ditransfer", res)
}




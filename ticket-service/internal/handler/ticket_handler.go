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


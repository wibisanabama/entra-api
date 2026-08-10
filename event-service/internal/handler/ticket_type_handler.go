package handler

import (
	"net/http"

	"entra-api/event-service/internal/service"
	"entra-api/shared/middleware"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
)

type TicketTypeHandler struct {
	eventService *service.EventService
}

func NewTicketTypeHandler(eventService *service.EventService) *TicketTypeHandler {
	return &TicketTypeHandler{eventService: eventService}
}

func (h *TicketTypeHandler) Create(c *gin.Context) {
	organizerID := c.GetString(middleware.AuthUserIDKey)
	eventID := c.Param("id")

	var req service.CreateTicketTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	ticketType, err := h.eventService.CreateTicketType(c.Request.Context(), organizerID, eventID, req)
	if err != nil {
		if err == service.ErrUnauthorized {
			response.Error(c, http.StatusForbidden, "not allowed to create ticket for this event")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "ticket type created", ticketType)
}

func (h *TicketTypeHandler) Update(c *gin.Context) {
	organizerID := c.GetString(middleware.AuthUserIDKey)
	eventID := c.Param("id")
	ticketID := c.Param("ticket_id")

	var req service.UpdateTicketTypeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	ticketType, err := h.eventService.UpdateTicketType(c.Request.Context(), organizerID, eventID, ticketID, req)
	if err != nil {
		if err == service.ErrUnauthorized {
			response.Error(c, http.StatusForbidden, "not allowed to update this ticket type")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "ticket type updated", ticketType)
}

func (h *TicketTypeHandler) Delete(c *gin.Context) {
	organizerID := c.GetString(middleware.AuthUserIDKey)
	eventID := c.Param("id")
	ticketID := c.Param("ticket_id")

	err := h.eventService.DeleteTicketType(c.Request.Context(), organizerID, eventID, ticketID)
	if err != nil {
		if err == service.ErrUnauthorized {
			response.Error(c, http.StatusForbidden, "not allowed to delete this ticket type")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "ticket type deleted", nil)
}

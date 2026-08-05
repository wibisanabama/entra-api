package handler

import (
	"errors"
	"net/http"
	"strconv"

	"entra-api/event-service/internal/service"
	"entra-api/shared/middleware"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
)

type EventHandler struct {
	eventService *service.EventService
}

func NewEventHandler(eventService *service.EventService) *EventHandler {
	return &EventHandler{eventService: eventService}
}

func (h *EventHandler) Create(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)

	var req service.CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	event, err := h.eventService.CreateEvent(c.Request.Context(), organizerID.(string), req)
	if err != nil {
		response.InternalError(c, "failed to create event")
		return
	}

	response.Success(c, http.StatusCreated, "event created successfully", event)
}

func (h *EventHandler) Get(c *gin.Context) {
	eventID := c.Param("id")

	event, err := h.eventService.GetEvent(c.Request.Context(), eventID)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			response.NotFound(c, "event not found")
			return
		}
		response.InternalError(c, "failed to get event")
		return
	}

	response.Success(c, http.StatusOK, "event retrieved", event)
}

func (h *EventHandler) ListTickets(c *gin.Context) {
	eventID := c.Param("id")

	tickets, err := h.eventService.ListTicketTypesForEvent(c.Request.Context(), eventID)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			response.NotFound(c, "event not found")
			return
		}
		response.InternalError(c, "failed to get tickets")
		return
	}

	response.Success(c, http.StatusOK, "tickets retrieved", tickets)
}

func (h *EventHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	events, total, err := h.eventService.ListEvents(c.Request.Context(), page, perPage)
	if err != nil {
		response.InternalError(c, "failed to list events")
		return
	}

	meta := response.NewMeta(page, perPage, total)
	response.SuccessWithPagination(c, "events retrieved", events, meta)
}

func (h *EventHandler) ListByOrganizer(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	events, total, err := h.eventService.ListEventsByOrganizer(c.Request.Context(), organizerID.(string), page, perPage)
	if err != nil {
		response.InternalError(c, "failed to list organizer events")
		return
	}

	meta := response.NewMeta(page, perPage, total)
	response.SuccessWithPagination(c, "events retrieved", events, meta)
}

func (h *EventHandler) Update(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)
	eventID := c.Param("id")

	var req service.UpdateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	event, err := h.eventService.UpdateEvent(c.Request.Context(), eventID, organizerID.(string), req)
	if err != nil {
		if errors.Is(err, service.ErrEventNotFound) {
			response.NotFound(c, "event not found")
			return
		}
		response.InternalError(c, "failed to update event")
		return
	}

	response.Success(c, http.StatusOK, "event updated", event)
}

func (h *EventHandler) Delete(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)
	eventID := c.Param("id")

	if err := h.eventService.DeleteEvent(c.Request.Context(), eventID, organizerID.(string)); err != nil {
		response.InternalError(c, "failed to delete event")
		return
	}

	response.Success(c, http.StatusOK, "event deleted", nil)
}

func (h *EventHandler) Search(c *gin.Context) {
	query := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	events, err := h.eventService.SearchEvents(c.Request.Context(), query, page, perPage)
	if err != nil {
		response.InternalError(c, "failed to search events")
		return
	}

	response.Success(c, http.StatusOK, "search results", events)
}


func (h *EventHandler) GetInternalEventIDs(c *gin.Context) {
	organizerID := c.Param("id")

	eventIDs, err := h.eventService.GetEventIDsByOrganizer(c.Request.Context(), organizerID)
	if err != nil {
		response.InternalError(c, "failed to get event IDs")
		return
	}

	response.Success(c, http.StatusOK, "event IDs retrieved", eventIDs)
}


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

type VenueHandler struct {
	venueService *service.VenueService
}

func NewVenueHandler(venueService *service.VenueService) *VenueHandler {
	return &VenueHandler{venueService: venueService}
}

func (h *VenueHandler) Create(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)

	var req service.CreateVenueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	venue, err := h.venueService.CreateVenue(c.Request.Context(), organizerID.(string), req)
	if err != nil {
		response.InternalError(c, "failed to create venue")
		return
	}

	response.Success(c, http.StatusCreated, "venue created successfully", venue)
}

func (h *VenueHandler) Get(c *gin.Context) {
	venueID := c.Param("id")

	venue, err := h.venueService.GetVenue(c.Request.Context(), venueID)
	if err != nil {
		if errors.Is(err, service.ErrVenueNotFound) {
			response.NotFound(c, "venue not found")
			return
		}
		response.InternalError(c, "failed to get venue")
		return
	}

	response.Success(c, http.StatusOK, "venue retrieved", venue)
}

func (h *VenueHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "10"))

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	venues, total, err := h.venueService.ListVenues(c.Request.Context(), page, perPage)
	if err != nil {
		response.InternalError(c, "failed to list venues")
		return
	}

	meta := response.NewMeta(page, perPage, total)
	response.SuccessWithPagination(c, "venues retrieved", venues, meta)
}

func (h *VenueHandler) Update(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)
	venueID := c.Param("id")

	var req service.UpdateVenueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	venue, err := h.venueService.UpdateVenue(c.Request.Context(), venueID, organizerID.(string), req)
	if err != nil {
		if errors.Is(err, service.ErrVenueNotFound) {
			response.NotFound(c, "venue not found")
			return
		}
		response.InternalError(c, "failed to update venue")
		return
	}

	response.Success(c, http.StatusOK, "venue updated", venue)
}

func (h *VenueHandler) Delete(c *gin.Context) {
	organizerID, _ := c.Get(middleware.AuthUserIDKey)
	venueID := c.Param("id")

	if err := h.venueService.DeleteVenue(c.Request.Context(), venueID, organizerID.(string)); err != nil {
		response.InternalError(c, "failed to delete venue")
		return
	}

	response.Success(c, http.StatusOK, "venue deleted", nil)
}

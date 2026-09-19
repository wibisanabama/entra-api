package handler

import (
	"errors"
	"net/http"
	"time"

	"entra-api/event-service/internal/repository/db"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type InternalTicketHandler struct {
	queries *db.Queries
}

func NewInternalTicketHandler(queries *db.Queries) *InternalTicketHandler {
	return &InternalTicketHandler{queries: queries}
}

type ReservationRequest struct {
	Quantity int32 `json:"quantity" binding:"required,min=1"`
}

func (h *InternalTicketHandler) ReserveTickets(c *gin.Context) {
	ticketTypeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.ValidationError(c, "invalid ticket type id")
		return
	}

	var req ReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	pgID := pgtype.UUID{Bytes: ticketTypeID, Valid: true}
	ticketType, err := h.queries.GetTicketTypeByID(c.Request.Context(), pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(c, http.StatusNotFound, "ticket type not found")
			return
		}
		response.InternalError(c, "failed to get ticket type")
		return
	}

	if !ticketType.IsActive {
		response.Error(c, http.StatusConflict, "ticket type is not active")
		return
	}

	now := time.Now()
	if ticketType.SaleStart.Valid && now.Before(ticketType.SaleStart.Time) {
		response.Error(c, http.StatusConflict, "ticket sales have not started yet")
		return
	}
	if ticketType.SaleEnd.Valid && now.After(ticketType.SaleEnd.Time) {
		response.Error(c, http.StatusConflict, "ticket sales period has ended")
		return
	}

	event, err := h.queries.GetEventByID(c.Request.Context(), ticketType.EventID)
	if err == nil {
		if event.Status != "published" {
			response.Error(c, http.StatusConflict, "event is not published")
			return
		}
		if event.EndDate.Valid && now.After(event.EndDate.Time) {
			response.Error(c, http.StatusConflict, "event has already ended")
			return
		}
	}

	ticket, err := h.queries.IncrementTicketSold(c.Request.Context(), db.IncrementTicketSoldParams{
		ID:   pgID,
		Sold: int32(req.Quantity),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(c, http.StatusConflict, "insufficient ticket inventory or invalid ticket type")
			return
		}
		response.InternalError(c, "failed to reserve tickets")
		return
	}

	response.Success(c, http.StatusOK, "tickets reserved", ticket)
}

func (h *InternalTicketHandler) ReleaseTickets(c *gin.Context) {
	ticketTypeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.ValidationError(c, "invalid ticket type id")
		return
	}

	var req ReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	pgID := pgtype.UUID{Bytes: ticketTypeID, Valid: true}
	ticket, err := h.queries.DecrementTicketSold(c.Request.Context(), db.DecrementTicketSoldParams{
		ID:   pgID,
		Sold: int32(req.Quantity),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.Error(c, http.StatusConflict, "cannot release more tickets than sold or invalid ticket type")
			return
		}
		response.InternalError(c, "failed to release tickets")
		return
	}

	response.Success(c, http.StatusOK, "tickets released", ticket)
}

func (h *InternalTicketHandler) GetTicketType(c *gin.Context) {
	ticketTypeID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.ValidationError(c, "invalid ticket type id")
		return
	}

	pgID := pgtype.UUID{Bytes: ticketTypeID, Valid: true}
	ticket, err := h.queries.GetTicketTypeByID(c.Request.Context(), pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			response.NotFound(c, "ticket type not found")
			return
		}
		response.InternalError(c, "failed to get ticket type")
		return
	}

	event, _ := h.queries.GetEventByID(c.Request.Context(), ticket.EventID)

	var saleStart *time.Time
	if ticket.SaleStart.Valid {
		saleStart = &ticket.SaleStart.Time
	}
	var saleEnd *time.Time
	if ticket.SaleEnd.Valid {
		saleEnd = &ticket.SaleEnd.Time
	}
	var eventEndDate *time.Time
	if event.EndDate.Valid {
		eventEndDate = &event.EndDate.Time
	}

	data := gin.H{
		"id":             uuid.UUID(ticket.ID.Bytes).String(),
		"event_id":       uuid.UUID(ticket.EventID.Bytes).String(),
		"name":           ticket.Name,
		"quantity":       ticket.Quantity,
		"sold":           ticket.Sold,
		"is_active":      ticket.IsActive,
		"sale_start":     saleStart,
		"sale_end":       saleEnd,
		"event_status":   event.Status,
		"event_end_date": eventEndDate,
	}

	response.Success(c, http.StatusOK, "ticket type retrieved", data)
}


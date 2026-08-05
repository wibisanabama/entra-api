package handler

import (
	"errors"
	"net/http"

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

	// In a real app we might want to do this in a transaction if reserving multiple ticket types,
	// but here we reserve one type at a time.
	pgID := pgtype.UUID{Bytes: ticketTypeID, Valid: true}
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

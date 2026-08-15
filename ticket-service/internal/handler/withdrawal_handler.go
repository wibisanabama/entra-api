package handler

import (
	"errors"
	"net/http"
	"strconv"

	"entra-api/shared/middleware"
	"entra-api/shared/response"
	"entra-api/ticket-service/internal/service"

	"github.com/gin-gonic/gin"
)

type WithdrawalHandler struct {
	ticketService *service.TicketService
}

func NewWithdrawalHandler(ticketService *service.TicketService) *WithdrawalHandler {
	return &WithdrawalHandler{ticketService: ticketService}
}

// GetOrganizerBalance handles GET /api/v1/tickets/organizer/balance
func (h *WithdrawalHandler) GetOrganizerBalance(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	balance, err := h.ticketService.GetOrganizerBalance(c.Request.Context(), organizerID.(string))
	if err != nil {
		response.InternalError(c, "failed to get balance: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "balance retrieved", balance)
}

// RequestWithdrawal handles POST /api/v1/tickets/organizer/withdrawals
func (h *WithdrawalHandler) RequestWithdrawal(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req service.CreateWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	withdrawal, err := h.ticketService.RequestWithdrawal(c.Request.Context(), organizerID.(string), req)
	if err != nil {
		if errors.Is(err, service.ErrInsufficientBalance) {
			response.Error(c, http.StatusBadRequest, "saldo tidak mencukupi untuk melakukan penarikan")
			return
		}
		if errors.Is(err, service.ErrInvalidAmount) {
			response.Error(c, http.StatusBadRequest, "minimal penarikan dana adalah Rp 10.000")
			return
		}
		response.InternalError(c, "failed to submit withdrawal request: "+err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "withdrawal request submitted successfully", withdrawal)
}

// ListOrganizerWithdrawals handles GET /api/v1/tickets/organizer/withdrawals
func (h *WithdrawalHandler) ListOrganizerWithdrawals(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	withdrawals, err := h.ticketService.ListOrganizerWithdrawals(c.Request.Context(), organizerID.(string), page, perPage)
	if err != nil {
		response.InternalError(c, "failed to list withdrawals: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "withdrawals retrieved", withdrawals)
}

// GetOrganizerWithdrawal handles GET /api/v1/tickets/organizer/withdrawals/:id
func (h *WithdrawalHandler) GetOrganizerWithdrawal(c *gin.Context) {
	organizerID, exists := c.Get(middleware.AuthUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized")
		return
	}

	withdrawalID := c.Param("id")
	withdrawal, err := h.ticketService.GetOrganizerWithdrawalDetail(c.Request.Context(), withdrawalID, organizerID.(string))
	if err != nil {
		if errors.Is(err, service.ErrWithdrawalNotFound) {
			response.Error(c, http.StatusNotFound, "withdrawal request not found")
			return
		}
		if errors.Is(err, service.ErrUnauthorizedAccess) {
			response.Error(c, http.StatusForbidden, "unauthorized access to this withdrawal")
			return
		}
		response.InternalError(c, "failed to get withdrawal: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "withdrawal retrieved", withdrawal)
}

// AdminListWithdrawals handles GET /api/v1/tickets/admin/withdrawals
func (h *WithdrawalHandler) AdminListWithdrawals(c *gin.Context) {
	role, exists := c.Get(middleware.AuthUserRoleKey)
	if !exists || (role.(string) != "admin" && role.(string) != "organizer") {
		// allow admin or fallback
	}

	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	perPage, _ := strconv.Atoi(c.DefaultQuery("per_page", "20"))

	withdrawals, err := h.ticketService.AdminListWithdrawals(c.Request.Context(), status, page, perPage)
	if err != nil {
		response.InternalError(c, "failed to list withdrawals: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "admin withdrawals list retrieved", withdrawals)
}

// AdminUpdateWithdrawalStatus handles PATCH /api/v1/tickets/admin/withdrawals/:id/status
func (h *WithdrawalHandler) AdminUpdateWithdrawalStatus(c *gin.Context) {
	withdrawalID := c.Param("id")

	var req service.UpdateWithdrawalStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	updated, err := h.ticketService.AdminUpdateWithdrawalStatus(c.Request.Context(), withdrawalID, req)
	if err != nil {
		response.InternalError(c, "failed to update withdrawal status: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "withdrawal status updated", updated)
}

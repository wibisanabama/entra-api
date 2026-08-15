package service

import (
	"context"
	"errors"
	"fmt"

	"entra-api/ticket-service/internal/repository/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrInsufficientBalance = errors.New("insufficient available balance for withdrawal")
	ErrWithdrawalNotFound  = errors.New("withdrawal request not found")
	ErrUnauthorizedAccess  = errors.New("unauthorized access to withdrawal request")
	ErrInvalidAmount       = errors.New("withdrawal amount must be at least 10,000")
)

type CreateWithdrawalRequest struct {
	Amount        float64 `json:"amount" binding:"required,min=10000"`
	BankName      string  `json:"bank_name" binding:"required"`
	AccountNumber string  `json:"account_number" binding:"required"`
	AccountName   string  `json:"account_name" binding:"required"`
	Notes         string  `json:"notes"`
}

type UpdateWithdrawalStatusRequest struct {
	Status          string `json:"status" binding:"required,oneof=APPROVED PAID REJECTED PENDING"`
	RejectionReason string `json:"rejection_reason"`
}

type OrganizerBalanceResponse struct {
	TotalRevenue     float64 `json:"total_revenue"`
	TotalWithdrawn   float64 `json:"total_withdrawn"`
	AvailableBalance float64 `json:"available_balance"`
	PendingAmount    float64 `json:"pending_amount"`
	PaidAmount       float64 `json:"paid_amount"`
	TotalRequests    int64   `json:"total_requests"`
}

func float64ToNumeric(val float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", val))
	return n
}

func numericToFloat64(n pgtype.Numeric) float64 {
	val, err := n.Float64Value()
	if err != nil || !val.Valid {
		return 0
	}
	return val.Float64
}

func (s *TicketService) GetOrganizerBalance(ctx context.Context, organizerID string) (*OrganizerBalanceResponse, error) {
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, fmt.Errorf("invalid organizer id: %w", err)
	}

	// 1. Get total revenue from successful orders across all events of this organizer
	stats, err := s.GetDashboardStats(ctx, organizerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch organizer stats: %w", err)
	}

	totalRevenue := numericToFloat64(stats.TotalRevenue)

	// 2. Get withdrawal summary for this organizer
	summary, err := s.queries.GetWithdrawnSummaryByOrganizer(ctx, orgUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch withdrawal summary: %w", err)
	}

	totalWithdrawn := numericToFloat64(summary.TotalDeducted)
	pendingAmount := numericToFloat64(summary.PendingAmount)
	paidAmount := numericToFloat64(summary.PaidAmount)
	availableBalance := totalRevenue - totalWithdrawn
	if availableBalance < 0 {
		availableBalance = 0
	}

	return &OrganizerBalanceResponse{
		TotalRevenue:     totalRevenue,
		TotalWithdrawn:   totalWithdrawn,
		AvailableBalance: availableBalance,
		PendingAmount:    pendingAmount,
		PaidAmount:       paidAmount,
		TotalRequests:    summary.TotalRequests,
	}, nil
}

func (s *TicketService) RequestWithdrawal(ctx context.Context, organizerID string, req CreateWithdrawalRequest) (*db.Withdrawal, error) {
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, fmt.Errorf("invalid organizer id: %w", err)
	}

	if req.Amount < 10000 {
		return nil, ErrInvalidAmount
	}

	// Verify balance
	balance, err := s.GetOrganizerBalance(ctx, organizerID)
	if err != nil {
		return nil, err
	}

	if req.Amount > balance.AvailableBalance {
		return nil, ErrInsufficientBalance
	}

	var notesPg pgtype.Text
	if req.Notes != "" {
		notesPg = pgtype.Text{String: req.Notes, Valid: true}
	}

	withdrawal, err := s.queries.CreateWithdrawal(ctx, db.CreateWithdrawalParams{
		OrganizerID:   orgUUID,
		Amount:        float64ToNumeric(req.Amount),
		BankName:      req.BankName,
		AccountNumber: req.AccountNumber,
		AccountName:   req.AccountName,
		Notes:         notesPg,
		Status:        "PENDING",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return &withdrawal, nil
}

func (s *TicketService) ListOrganizerWithdrawals(ctx context.Context, organizerID string, page, perPage int) ([]db.Withdrawal, error) {
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, fmt.Errorf("invalid organizer id: %w", err)
	}

	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	return s.queries.ListWithdrawalsByOrganizer(ctx, db.ListWithdrawalsByOrganizerParams{
		OrganizerID: orgUUID,
		Limit:       int32(perPage),
		Offset:      int32(offset),
	})
}

func (s *TicketService) GetOrganizerWithdrawalDetail(ctx context.Context, withdrawalID string, organizerID string) (*db.Withdrawal, error) {
	wUUID, err := uuid.Parse(withdrawalID)
	if err != nil {
		return nil, fmt.Errorf("invalid withdrawal id: %w", err)
	}
	orgUUID, err := uuid.Parse(organizerID)
	if err != nil {
		return nil, fmt.Errorf("invalid organizer id: %w", err)
	}

	withdrawal, err := s.queries.GetWithdrawal(ctx, wUUID)
	if err != nil {
		return nil, ErrWithdrawalNotFound
	}

	if withdrawal.OrganizerID != orgUUID {
		return nil, ErrUnauthorizedAccess
	}

	return &withdrawal, nil
}

func (s *TicketService) AdminListWithdrawals(ctx context.Context, status string, page, perPage int) ([]db.Withdrawal, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	if status != "" {
		return s.queries.ListWithdrawalsByStatus(ctx, db.ListWithdrawalsByStatusParams{
			Status: status,
			Limit:  int32(perPage),
			Offset: int32(offset),
		})
	}

	return s.queries.ListAllWithdrawals(ctx, db.ListAllWithdrawalsParams{
		Limit:  int32(perPage),
		Offset: int32(offset),
	})
}

func (s *TicketService) AdminUpdateWithdrawalStatus(ctx context.Context, withdrawalID string, req UpdateWithdrawalStatusRequest) (*db.Withdrawal, error) {
	wUUID, err := uuid.Parse(withdrawalID)
	if err != nil {
		return nil, fmt.Errorf("invalid withdrawal id: %w", err)
	}

	var reasonPg pgtype.Text
	if req.RejectionReason != "" {
		reasonPg = pgtype.Text{String: req.RejectionReason, Valid: true}
	}

	updated, err := s.queries.UpdateWithdrawalStatus(ctx, db.UpdateWithdrawalStatusParams{
		ID:              wUUID,
		Status:          req.Status,
		RejectionReason: reasonPg,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update withdrawal: %w", err)
	}

	return &updated, nil
}

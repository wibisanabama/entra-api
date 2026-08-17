package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"entra-api/cashless-service/internal/repository/db"
	"entra-api/shared/kafka"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type WalletService struct {
	queries  *db.Queries
	producer *kafka.Producer
}

func NewWalletService(queries *db.Queries, producer *kafka.Producer) *WalletService {
	return &WalletService{queries: queries, producer: producer}
}

func (s *WalletService) GetWallet(ctx context.Context, userID string) (*db.Wallet, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, err
	}

	wallet, err := s.queries.GetWalletByUserID(ctx, uid)
	if err != nil {
		// Auto-create if not found
		wallet, err = s.queries.CreateWallet(ctx, uid)
		if err != nil {
			return nil, err
		}
	}

	return &wallet, nil
}

func (s *WalletService) InitiateTopUp(ctx context.Context, userID string, amount float64) (*db.Topup, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))

	topup, err := s.queries.CreateTopup(ctx, db.CreateTopupParams{
		WalletID: wallet.ID,
		Amount:   amt,
		Status:   "PENDING",
	})
	if err != nil {
		return nil, err
	}

	// Publish to payment service
	payload := map[string]interface{}{
		"reference_id":   topup.ID.String(),
		"reference_type": "TOPUP",
		"user_id":        userID,
		"amount":         amount,
	}
	payloadBytes, _ := json.Marshal(payload)
	_ = s.producer.Publish(ctx, "topup.created", []byte(topup.ID.String()), payloadBytes)

	return &topup, nil
}

func (s *WalletService) ProcessTopUpSuccess(ctx context.Context, topupID string) error {
	tid, err := uuid.Parse(topupID)
	if err != nil {
		return err
	}

	topup, err := s.queries.GetTopup(ctx, tid)
	if err != nil {
		return err
	}

	if topup.Status != "PENDING" {
		return nil // already processed
	}

	_, err = s.queries.UpdateTopupStatus(ctx, tid, "SUCCESS")
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateWalletBalance(ctx, topup.WalletID, topup.Amount)
	if err != nil {
		return err
	}

	_, err = s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    topup.WalletID,
		Type:        "CREDIT",
		Amount:      topup.Amount,
		MerchantID:  uuid.NullUUID{Valid: false},
		Description: pgtype.Text{String: "Wallet Top-up", Valid: true},
	})

	return err
}

func (s *WalletService) ProcessTopUpFailed(ctx context.Context, topupID string) error {
	tid, err := uuid.Parse(topupID)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateTopupStatus(ctx, tid, "FAILED")
	return err
}

func (s *WalletService) PayAtMerchant(ctx context.Context, userID string, amount float64, merchantID string) (*db.Transaction, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Read balance
	var balance float64
	// Small hack to convert pgtype.Numeric to float64 for comparison
	val, _ := wallet.Balance.Value()
	if vStr, ok := val.(string); ok {
		fmt.Sscanf(vStr, "%f", &balance)
	}

	if balance < amount {
		return nil, errors.New("insufficient balance")
	}

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))
	var negAmt pgtype.Numeric
	_ = negAmt.Scan(fmt.Sprintf("-%f", amount))

	// Update balance (decrement)
	_, err = s.queries.UpdateWalletBalance(ctx, wallet.ID, negAmt)
	if err != nil {
		return nil, err
	}

	mid, _ := uuid.Parse(merchantID)

	tx, err := s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    wallet.ID,
		Type:        "DEBIT",
		Amount:      amt,
		MerchantID:  uuid.NullUUID{UUID: mid, Valid: merchantID != ""},
		Description: pgtype.Text{String: "Purchase at merchant", Valid: true},
	})
	
	return &tx, err
}

type RefundRequest struct {
	Amount        float64 `json:"amount" binding:"required,min=1000"`
	BankName      string  `json:"bank_name" binding:"required"`
	AccountNumber string  `json:"account_number" binding:"required"`
	AccountHolder string  `json:"account_holder" binding:"required"`
	Reason        string  `json:"reason"`
}

func (s *WalletService) RequestRefund(ctx context.Context, userID string, amount float64, bankName, accountNumber, accountHolder, reason string) (*db.Transaction, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	var balance float64
	val, _ := wallet.Balance.Value()
	if vStr, ok := val.(string); ok {
		fmt.Sscanf(vStr, "%f", &balance)
	}

	if balance < amount {
		return nil, errors.New("saldo tidak mencukupi untuk melakukan refund")
	}

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))
	var negAmt pgtype.Numeric
	_ = negAmt.Scan(fmt.Sprintf("-%f", amount))

	// Update balance (decrement)
	_, err = s.queries.UpdateWalletBalance(ctx, wallet.ID, negAmt)
	if err != nil {
		return nil, err
	}

	desc := fmt.Sprintf("Refund saldo ke %s %s a/n %s", bankName, accountNumber, accountHolder)
	if reason != "" {
		desc += fmt.Sprintf(" (%s)", reason)
	}

	tx, err := s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    wallet.ID,
		Type:        "DEBIT",
		Amount:      amt,
		MerchantID:  uuid.NullUUID{Valid: false},
		Description: pgtype.Text{String: desc, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	// Publish event to Kafka
	payload := map[string]interface{}{
		"transaction_id": tx.ID.String(),
		"wallet_id":      wallet.ID.String(),
		"user_id":        userID,
		"amount":         amount,
		"bank_name":      bankName,
		"account_number": accountNumber,
		"account_holder": accountHolder,
	}
	payloadBytes, _ := json.Marshal(payload)
	_ = s.producer.Publish(ctx, "cashless.refund", []byte(tx.ID.String()), payloadBytes)

	return &tx, nil
}

func (s *WalletService) GetTransactions(ctx context.Context, userID string) ([]db.Transaction, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.queries.ListTransactions(ctx, wallet.ID, 50, 0)
}


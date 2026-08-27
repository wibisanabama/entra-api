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
	"github.com/jackc/pgx/v5/pgxpool"
)

type WalletService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	producer *kafka.Producer
}

func NewWalletService(pool *pgxpool.Pool, queries *db.Queries, producer *kafka.Producer) *WalletService {
	return &WalletService{pool: pool, queries: queries, producer: producer}
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
	if s.producer != nil {
		payload := map[string]interface{}{
			"reference_id":   topup.ID.String(),
			"reference_type": "TOPUP",
			"user_id":        userID,
			"amount":         amount,
		}
		payloadBytes, _ := json.Marshal(payload)
		_ = s.producer.Publish(ctx, "topup.created", []byte(topup.ID.String()), payloadBytes)
	}

	return &topup, nil
}

func (s *WalletService) ProcessTopUpSuccess(ctx context.Context, topupID string) error {
	tid, err := uuid.Parse(topupID)
	if err != nil {
		return err
	}

	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		qtx := s.queries.WithTx(tx)
		topup, err := qtx.GetTopup(ctx, tid)
		if err != nil {
			return err
		}

		if topup.Status != "PENDING" {
			return nil // already processed
		}

		_, err = qtx.UpdateTopupStatus(ctx, tid, "SUCCESS")
		if err != nil {
			return err
		}

		_, err = qtx.UpdateWalletBalance(ctx, topup.WalletID, topup.Amount)
		if err != nil {
			return err
		}

		_, err = qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    topup.WalletID,
			Type:        "CREDIT",
			Amount:      topup.Amount,
			MerchantID:  uuid.NullUUID{Valid: false},
			Description: pgtype.Text{String: "Wallet Top-up", Valid: true},
		})
		if err != nil {
			return err
		}

		return tx.Commit(ctx)
	}

	// Fallback if pool is nil
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

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))
	mid, _ := uuid.Parse(merchantID)

	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		qtx := s.queries.WithTx(tx)

		// Atomic balance deduction with database-level condition (balance >= amount)
		_, err = qtx.DeductWalletBalance(ctx, wallet.ID, amt)
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan transaksi")
		}

		txn, err := qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  uuid.NullUUID{UUID: mid, Valid: merchantID != ""},
			Description: pgtype.Text{String: "Purchase at merchant", Valid: true},
		})
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}

		return &txn, nil
	}

	// Fallback if pool is nil
	_, err = s.queries.DeductWalletBalance(ctx, wallet.ID, amt)
	if err != nil {
		return nil, errors.New("saldo tidak mencukupi untuk melakukan transaksi")
	}

	txn, err := s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    wallet.ID,
		Type:        "DEBIT",
		Amount:      amt,
		MerchantID:  uuid.NullUUID{UUID: mid, Valid: merchantID != ""},
		Description: pgtype.Text{String: "Purchase at merchant", Valid: true},
	})
	
	return &txn, err
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

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))

	desc := fmt.Sprintf("Refund saldo ke %s %s a/n %s", bankName, accountNumber, accountHolder)
	if reason != "" {
		desc += fmt.Sprintf(" (%s)", reason)
	}

	var txn db.Transaction
	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		qtx := s.queries.WithTx(tx)

		// Atomic balance deduction with database-level condition (balance >= amount)
		_, err = qtx.DeductWalletBalance(ctx, wallet.ID, amt)
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan refund")
		}

		txn, err = qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  uuid.NullUUID{Valid: false},
			Description: pgtype.Text{String: desc, Valid: true},
		})
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	} else {
		_, err = s.queries.DeductWalletBalance(ctx, wallet.ID, amt)
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan refund")
		}

		txn, err = s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  uuid.NullUUID{Valid: false},
			Description: pgtype.Text{String: desc, Valid: true},
		})
		if err != nil {
			return nil, err
		}
	}

	// Publish event to Kafka
	if s.producer != nil {
		payload := map[string]interface{}{
			"transaction_id": txn.ID.String(),
			"wallet_id":      wallet.ID.String(),
			"user_id":        userID,
			"amount":         amount,
			"bank_name":      bankName,
			"account_number": accountNumber,
			"account_holder": accountHolder,
		}
		payloadBytes, _ := json.Marshal(payload)
		_ = s.producer.Publish(ctx, "cashless.refund", []byte(txn.ID.String()), payloadBytes)
	}

	return &txn, nil
}

func (s *WalletService) GetTransactions(ctx context.Context, userID string) ([]db.Transaction, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.queries.ListTransactions(ctx, wallet.ID, 50, 0)
}



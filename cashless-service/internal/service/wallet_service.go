package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"entra-api/cashless-service/internal/repository/db"
	"entra-api/shared/kafka"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"github.com/midtrans/midtrans-go/snap"
)

type WalletService struct {
	pool       *pgxpool.Pool
	queries    *db.Queries
	producer   *kafka.Producer
	snapClient snap.Client
	coreClient coreapi.Client
}

func NewWalletService(pool *pgxpool.Pool, queries *db.Queries, producer *kafka.Producer) *WalletService {
	serverKey := os.Getenv("MIDTRANS_SERVER_KEY")
	if serverKey == "" {
		serverKey = "SB-Mid-server-dummy-key-for-dev-only"
	}

	var sClient snap.Client
	sClient.New(serverKey, midtrans.Sandbox)

	var cClient coreapi.Client
	cClient.New(serverKey, midtrans.Sandbox)

	return &WalletService{
		pool:       pool,
		queries:    queries,
		producer:   producer,
		snapClient: sClient,
		coreClient: cClient,
	}
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

type TopUpResult struct {
	Topup       *db.Topup `json:"topup"`
	Token       string    `json:"token"`
	RedirectURL string    `json:"redirect_url"`
}

func (s *WalletService) InitiateTopUp(ctx context.Context, userID string, amount float64) (*TopUpResult, error) {
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

	midtransOrderID := fmt.Sprintf("TOPUP_%s", topup.ID.String())

	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
			OrderID:  midtransOrderID,
			GrossAmt: int64(amount),
		},
		CreditCard: &snap.CreditCardDetails{
			Secure: true,
		},
	}

	snapResp, snapErr := s.snapClient.CreateTransaction(req)
	token := ""
	redirectURL := ""
	if snapErr != nil {
		slog.Warn("Midtrans Snap transaction creation failed in non-production, returning mock token", "error", snapErr, "topup_id", topup.ID.String())
		token = "MOCK_SNAP_" + midtransOrderID
		redirectURL = fmt.Sprintf("https://sandbox.entra.local/pay/%s", topup.ID.String())
	} else {
		token = snapResp.Token
		redirectURL = snapResp.RedirectURL
	}

	return &TopUpResult{
		Topup:       &topup,
		Token:       token,
		RedirectURL: redirectURL,
	}, nil
}

func (s *WalletService) ConfirmTopUp(ctx context.Context, userID string, topupID string) (*db.Wallet, error) {
	tid, err := uuid.Parse(topupID)
	if err != nil {
		return nil, errors.New("invalid topup id")
	}

	topup, err := s.queries.GetTopup(ctx, tid)
	if err != nil {
		return nil, errors.New("topup record not found")
	}

	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	if topup.WalletID != wallet.ID {
		return nil, errors.New("access denied: topup does not belong to user wallet")
	}

	if err := s.ProcessTopUpSuccess(ctx, topupID); err != nil {
		return nil, err
	}

	updatedWallet, err := s.queries.GetWalletByUserID(ctx, wallet.UserID)
	if err != nil {
		return wallet, nil
	}

	return &updatedWallet, nil
}

func (s *WalletService) HandleMidtransNotification(ctx context.Context, payload map[string]interface{}) error {
	rawOrderID, ok := payload["order_id"].(string)
	if !ok {
		return errors.New("invalid order_id in payload")
	}

	parts := strings.Split(rawOrderID, "_")
	var topupID string
	if len(parts) >= 2 && parts[0] == "TOPUP" {
		topupID = parts[1]
	} else {
		topupID = parts[0]
	}

	tx, coreErr := s.coreClient.CheckTransaction(rawOrderID)
	if coreErr != nil {
		txStatus, _ := payload["transaction_status"].(string)
		if txStatus == "settlement" || txStatus == "capture" {
			return s.ProcessTopUpSuccess(ctx, topupID)
		} else if txStatus == "cancel" || txStatus == "deny" || txStatus == "expire" {
			return s.ProcessTopUpFailed(ctx, topupID)
		}
		return coreErr
	}

	switch tx.TransactionStatus {
	case "capture":
		if tx.FraudStatus == "accept" {
			return s.ProcessTopUpSuccess(ctx, topupID)
		}
	case "settlement":
		return s.ProcessTopUpSuccess(ctx, topupID)
	case "cancel", "deny", "expire":
		return s.ProcessTopUpFailed(ctx, topupID)
	}

	return nil
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

		qtx := db.New(tx)
		topup, err := qtx.GetTopup(ctx, tid)
		if err != nil {
			return err
		}

		if topup.Status != "PENDING" {
			return nil // already processed
		}

		_, err = qtx.UpdateTopupStatus(ctx, db.UpdateTopupStatusParams{
			ID:     tid,
			Status: "SUCCESS",
		})
		if err != nil {
			return err
		}

		_, err = qtx.UpdateWalletBalance(ctx, db.UpdateWalletBalanceParams{
			ID:      topup.WalletID,
			Balance: topup.Amount,
		})
		if err != nil {
			return err
		}

		_, err = qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    topup.WalletID,
			Type:        "CREDIT",
			Amount:      topup.Amount,
			MerchantID:  pgtype.UUID{Valid: false},
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

	_, err = s.queries.UpdateTopupStatus(ctx, db.UpdateTopupStatusParams{
		ID:     tid,
		Status: "SUCCESS",
	})
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateWalletBalance(ctx, db.UpdateWalletBalanceParams{
		ID:      topup.WalletID,
		Balance: topup.Amount,
	})
	if err != nil {
		return err
	}

	_, err = s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    topup.WalletID,
		Type:        "CREDIT",
		Amount:      topup.Amount,
		MerchantID:  pgtype.UUID{Valid: false},
		Description: pgtype.Text{String: "Wallet Top-up", Valid: true},
	})

	return err
}

func (s *WalletService) ProcessTopUpFailed(ctx context.Context, topupID string) error {
	tid, err := uuid.Parse(topupID)
	if err != nil {
		return err
	}

	_, err = s.queries.UpdateTopupStatus(ctx, db.UpdateTopupStatusParams{
		ID:     tid,
		Status: "FAILED",
	})
	return err
}

func (s *WalletService) PayAtMerchant(ctx context.Context, userID string, amount float64, merchantID string, merchantName string) (*db.Transaction, error) {
	wallet, err := s.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}

	var amt pgtype.Numeric
	_ = amt.Scan(fmt.Sprintf("%f", amount))
	mid, _ := uuid.Parse(merchantID)
	hasValidUUID := mid != uuid.Nil

	desc := "Pembayaran di merchant"
	if merchantName != "" {
		desc = fmt.Sprintf("Belanja di %s", merchantName)
	}

	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		qtx := db.New(tx)

		// Atomic balance deduction with database-level condition (balance >= amount)
		_, err = qtx.DeductWalletBalance(ctx, db.DeductWalletBalanceParams{
			ID:      wallet.ID,
			Balance: amt,
		})
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan transaksi")
		}

		txn, err := qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  pgtype.UUID{Bytes: mid, Valid: hasValidUUID},
			Description: pgtype.Text{String: desc, Valid: true},
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
	_, err = s.queries.DeductWalletBalance(ctx, db.DeductWalletBalanceParams{
		ID:      wallet.ID,
		Balance: amt,
	})
	if err != nil {
		return nil, errors.New("saldo tidak mencukupi untuk melakukan transaksi")
	}

	txn, err := s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
		WalletID:    wallet.ID,
		Type:        "DEBIT",
		Amount:      amt,
		MerchantID:  pgtype.UUID{Bytes: mid, Valid: hasValidUUID},
		Description: pgtype.Text{String: desc, Valid: true},
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

		qtx := db.New(tx)

		// Atomic balance deduction with database-level condition (balance >= amount)
		_, err = qtx.DeductWalletBalance(ctx, db.DeductWalletBalanceParams{
			ID:      wallet.ID,
			Balance: amt,
		})
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan refund")
		}

		txn, err = qtx.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  pgtype.UUID{Valid: false},
			Description: pgtype.Text{String: desc, Valid: true},
		})
		if err != nil {
			return nil, err
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	} else {
		_, err = s.queries.DeductWalletBalance(ctx, db.DeductWalletBalanceParams{
			ID:      wallet.ID,
			Balance: amt,
		})
		if err != nil {
			return nil, errors.New("saldo tidak mencukupi untuk melakukan refund")
		}

		txn, err = s.queries.CreateTransaction(ctx, db.CreateTransactionParams{
			WalletID:    wallet.ID,
			Type:        "DEBIT",
			Amount:      amt,
			MerchantID:  pgtype.UUID{Valid: false},
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
	return s.queries.ListTransactions(ctx, db.ListTransactionsParams{
		WalletID: wallet.ID,
		Limit:    50,
		Offset:   0,
	})
}



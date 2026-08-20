package db

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateTopupParams struct {
	WalletID uuid.UUID      `json:"wallet_id"`
	Amount   pgtype.Numeric `json:"amount"`
	Status   string         `json:"status"`
}

func (q *Queries) CreateTopup(ctx context.Context, arg CreateTopupParams) (Topup, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO topups (wallet_id, amount, status) VALUES ($1, $2, $3) RETURNING id, wallet_id, amount, status, created_at, updated_at`,
		arg.WalletID, arg.Amount, arg.Status,
	)
	var i Topup
	err := row.Scan(&i.ID, &i.WalletID, &i.Amount, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

type CreateTransactionParams struct {
	WalletID    uuid.UUID      `json:"wallet_id"`
	Type        string         `json:"type"`
	Amount      pgtype.Numeric `json:"amount"`
	MerchantID  uuid.NullUUID  `json:"merchant_id"`
	Description pgtype.Text    `json:"description"`
}

func (q *Queries) CreateTransaction(ctx context.Context, arg CreateTransactionParams) (Transaction, error) {
	row := q.db.QueryRow(ctx,
		`INSERT INTO transactions (wallet_id, type, amount, merchant_id, description) VALUES ($1, $2, $3, $4, $5) RETURNING id, wallet_id, type, amount, merchant_id, description, created_at`,
		arg.WalletID, arg.Type, arg.Amount, arg.MerchantID, arg.Description,
	)
	var i Transaction
	err := row.Scan(&i.ID, &i.WalletID, &i.Type, &i.Amount, &i.MerchantID, &i.Description, &i.CreatedAt)
	return i, err
}

func (q *Queries) CreateWallet(ctx context.Context, userID uuid.UUID) (Wallet, error) {
	row := q.db.QueryRow(ctx, `INSERT INTO wallets (user_id) VALUES ($1) RETURNING id, user_id, balance, created_at, updated_at`, userID)
	var i Wallet
	err := row.Scan(&i.ID, &i.UserID, &i.Balance, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetTopup(ctx context.Context, id uuid.UUID) (Topup, error) {
	row := q.db.QueryRow(ctx, `SELECT id, wallet_id, amount, status, created_at, updated_at FROM topups WHERE id = $1`, id)
	var i Topup
	err := row.Scan(&i.ID, &i.WalletID, &i.Amount, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) GetWalletByUserID(ctx context.Context, userID uuid.UUID) (Wallet, error) {
	row := q.db.QueryRow(ctx, `SELECT id, user_id, balance, created_at, updated_at FROM wallets WHERE user_id = $1`, userID)
	var i Wallet
	err := row.Scan(&i.ID, &i.UserID, &i.Balance, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) ListTransactions(ctx context.Context, walletID uuid.UUID, limit int32, offset int32) ([]Transaction, error) {
	rows, err := q.db.Query(ctx, `SELECT id, wallet_id, type, amount, merchant_id, description FROM transactions WHERE wallet_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Transaction{}
	for rows.Next() {
		var i Transaction
		if err := rows.Scan(&i.ID, &i.WalletID, &i.Type, &i.Amount, &i.MerchantID, &i.Description, &i.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

func (q *Queries) UpdateTopupStatus(ctx context.Context, id uuid.UUID, status string) (Topup, error) {
	row := q.db.QueryRow(ctx, `UPDATE topups SET status = $2, updated_at = NOW() WHERE id = $1 RETURNING id, wallet_id, amount, status, created_at, updated_at`, id, status)
	var i Topup
	err := row.Scan(&i.ID, &i.WalletID, &i.Amount, &i.Status, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) UpdateWalletBalance(ctx context.Context, id uuid.UUID, balance pgtype.Numeric) (Wallet, error) {
	row := q.db.QueryRow(ctx, `UPDATE wallets SET balance = balance + $2, updated_at = NOW() WHERE id = $1 RETURNING id, user_id, balance, created_at, updated_at`, id, balance)
	var i Wallet
	err := row.Scan(&i.ID, &i.UserID, &i.Balance, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func (q *Queries) DeductWalletBalance(ctx context.Context, id uuid.UUID, amount pgtype.Numeric) (Wallet, error) {
	row := q.db.QueryRow(ctx, `UPDATE wallets SET balance = balance - $2, updated_at = NOW() WHERE id = $1 AND balance >= $2 RETURNING id, user_id, balance, created_at, updated_at`, id, amount)
	var i Wallet
	err := row.Scan(&i.ID, &i.UserID, &i.Balance, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

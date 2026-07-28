package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Topup struct {
	ID        uuid.UUID      `json:"id"`
	WalletID  uuid.UUID      `json:"wallet_id"`
	Amount    pgtype.Numeric `json:"amount"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type Transaction struct {
	ID          uuid.UUID      `json:"id"`
	WalletID    uuid.UUID      `json:"wallet_id"`
	Type        string         `json:"type"`
	Amount      pgtype.Numeric `json:"amount"`
	MerchantID  uuid.NullUUID  `json:"merchant_id"`
	Description pgtype.Text    `json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
}

type Wallet struct {
	ID        uuid.UUID      `json:"id"`
	UserID    uuid.UUID      `json:"user_id"`
	Balance   pgtype.Numeric `json:"balance"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

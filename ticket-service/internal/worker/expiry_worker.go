package worker

import (
	"context"
	"log/slog"
	"time"

	"entra-api/ticket-service/internal/repository/db"
	"entra-api/ticket-service/internal/service"
)

type ExpiryWorker struct {
	queries       *db.Queries
	ticketService *service.TicketService
}

func NewExpiryWorker(queries *db.Queries, ticketService *service.TicketService) *ExpiryWorker {
	return &ExpiryWorker{queries: queries, ticketService: ticketService}
}

func (w *ExpiryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processExpiredOrders(ctx)
		}
	}
}

func (w *ExpiryWorker) processExpiredOrders(ctx context.Context) {
	orders, err := w.queries.GetExpiredPendingOrders(ctx)
	if err != nil {
		slog.Error("failed to get expired orders", "error", err)
		return
	}

	for _, order := range orders {
		err := w.ticketService.CancelOrder(ctx, order.ID.String())
		if err != nil {
			slog.Error("failed to cancel expired order", "order_id", order.ID, "error", err)
		} else {
			slog.Info("cancelled expired order", "order_id", order.ID)
		}
	}
}

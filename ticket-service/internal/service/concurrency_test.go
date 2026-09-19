package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entra-api/ticket-service/internal/service"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func getTestRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}

func TestRedisLuaAtomicReservationAndRelease(t *testing.T) {
	rdb, err := getTestRedisClient()
	if err != nil {
		t.Skipf("skipping redis test: redis not available: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	ticketTypeID := uuid.New().String()
	stockKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)

	// Set initial stock to 5
	if err := rdb.Set(ctx, stockKey, 5, 0).Err(); err != nil {
		t.Fatalf("failed to set initial stock: %v", err)
	}
	defer rdb.Del(ctx, stockKey)

	svc := service.NewTicketService(nil, nil, nil, nil, rdb)

	// Reserve 3 tickets -> should succeed
	err = svc.ReserveTicketStock(ctx, ticketTypeID, 3)
	if err != nil {
		t.Fatalf("expected successful reservation of 3 tickets, got: %v", err)
	}

	// Verify remaining stock is 2
	rem, _ := rdb.Get(ctx, stockKey).Int()
	if rem != 2 {
		t.Errorf("expected remaining stock 2, got %d", rem)
	}

	// Try reserving 3 tickets -> should fail (only 2 left)
	err = svc.ReserveTicketStock(ctx, ticketTypeID, 3)
	if !errors.Is(err, service.ErrSoldOut) {
		t.Fatalf("expected ErrSoldOut, got %v", err)
	}

	// Stock must still be 2
	rem, _ = rdb.Get(ctx, stockKey).Int()
	if rem != 2 {
		t.Errorf("expected remaining stock still 2, got %d", rem)
	}

	// Reserve remaining 2 -> should succeed
	err = svc.ReserveTicketStock(ctx, ticketTypeID, 2)
	if err != nil {
		t.Fatalf("expected successful reservation of 2 tickets, got: %v", err)
	}

	// Stock is now 0
	rem, _ = rdb.Get(ctx, stockKey).Int()
	if rem != 0 {
		t.Errorf("expected remaining stock 0, got %d", rem)
	}

	// Reserve 1 -> must fail with ErrSoldOut
	err = svc.ReserveTicketStock(ctx, ticketTypeID, 1)
	if !errors.Is(err, service.ErrSoldOut) {
		t.Fatalf("expected ErrSoldOut, got %v", err)
	}

	// Release 2 tickets back
	svc.ReleaseTicketStock(ctx, ticketTypeID, 2)

	// Stock is now 2
	rem, _ = rdb.Get(ctx, stockKey).Int()
	if rem != 2 {
		t.Errorf("expected stock 2 after release, got %d", rem)
	}
}

func TestHighConcurrencyStockReservationRaceCondition(t *testing.T) {
	rdb, err := getTestRedisClient()
	if err != nil {
		t.Skipf("skipping redis test: redis not available: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	ticketTypeID := uuid.New().String()
	stockKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)

	// Total stock available: 20
	const initialStock = 20
	const totalRivals = 100 // 100 concurrent requests competing for 20 tickets

	if err := rdb.Set(ctx, stockKey, initialStock, 0).Err(); err != nil {
		t.Fatalf("failed to set initial stock: %v", err)
	}
	defer rdb.Del(ctx, stockKey)

	svc := service.NewTicketService(nil, nil, nil, nil, rdb)

	var successCount int64
	var soldOutCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < totalRivals; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier // Wait until all goroutines are ready

			err := svc.ReserveTicketStock(ctx, ticketTypeID, 1)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else if errors.Is(err, service.ErrSoldOut) {
				atomic.AddInt64(&soldOutCount, 1)
			}
		}()
	}

	// Fire all goroutines simultaneously
	close(startBarrier)
	wg.Wait()

	// Strict verification: Zero overselling!
	if successCount != initialStock {
		t.Errorf("Overselling or underselling detected! Expected exactly %d successes, got %d", initialStock, successCount)
	}

	expectedSoldOut := int64(totalRivals - initialStock)
	if soldOutCount != expectedSoldOut {
		t.Errorf("Expected exactly %d sold-out rejections, got %d", expectedSoldOut, soldOutCount)
	}

	// Final stock in Redis must be exactly 0
	rem, err := rdb.Get(ctx, stockKey).Int()
	if err != nil || rem != 0 {
		t.Errorf("Expected final stock 0, got %d (err: %v)", rem, err)
	}
}

func TestHighConcurrency1000Rivals(t *testing.T) {
	rdb, err := getTestRedisClient()
	if err != nil {
		t.Skipf("skipping redis test: redis not available: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	ticketTypeID := uuid.New().String()
	stockKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)

	const quota = 50
	const rivals = 1000 // 1000 concurrent goroutines competing for 50 tickets

	if err := rdb.Set(ctx, stockKey, quota, 0).Err(); err != nil {
		t.Fatalf("failed to set initial stock: %v", err)
	}
	defer rdb.Del(ctx, stockKey)

	svc := service.NewTicketService(nil, nil, nil, nil, rdb)

	var successCount int64
	var soldOutCount int64

	startBarrier := make(chan struct{})
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < rivals; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			err := svc.ReserveTicketStock(ctx, ticketTypeID, 1)
			if err == nil {
				atomic.AddInt64(&successCount, 1)
			} else if errors.Is(err, service.ErrSoldOut) {
				atomic.AddInt64(&soldOutCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()
	duration := time.Since(startTime)

	t.Logf("Processed %d concurrent ticket reservations in %v (avg %.2f req/s)", rivals, duration, float64(rivals)/duration.Seconds())

	// Exact assertions: 0 oversold
	if successCount != quota {
		t.Fatalf("CRITICAL: Overselling detected! Expected %d, got %d", quota, successCount)
	}

	expectedRejections := int64(rivals - quota)
	if soldOutCount != expectedRejections {
		t.Fatalf("Expected %d sold-out rejections, got %d", expectedRejections, soldOutCount)
	}

	// Verify stock is 0
	rem, _ := rdb.Get(ctx, stockKey).Int()
	if rem != 0 {
		t.Fatalf("Expected remaining stock 0, got %d", rem)
	}
}

func BenchmarkReserveTicketStock(b *testing.B) {
	rdb, err := getTestRedisClient()
	if err != nil {
		b.Skipf("skipping redis benchmark: %v", err)
	}
	defer rdb.Close()

	ctx := context.Background()
	ticketTypeID := uuid.New().String()
	stockKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)

	_ = rdb.Set(ctx, stockKey, b.N, 0).Err()
	defer rdb.Del(ctx, stockKey)

	svc := service.NewTicketService(nil, nil, nil, nil, rdb)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = svc.ReserveTicketStock(ctx, ticketTypeID, 1)
		}
	})
}


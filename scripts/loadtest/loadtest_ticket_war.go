package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var reserveStockLua = redis.NewScript(`
local stock = tonumber(redis.call('GET', KEYS[1]))
if stock == nil then
    return -1
end
local qty = tonumber(ARGV[1])
if stock >= qty then
    redis.call('DECRBY', KEYS[1], qty)
    return 1
else
    return 0
end
`)

var releaseStockLua = redis.NewScript(`
local exists = redis.call('EXISTS', KEYS[1])
if exists == 1 then
    redis.call('INCRBY', KEYS[1], ARGV[1])
    return 1
else
    return 0
end
`)

func main() {
	fmt.Println("==================================================================")
	fmt.Println("   ENTRA HIGH CONCURRENCY LOAD TEST: TICKET WAR (FLASH SALE)")
	fmt.Println("==================================================================")

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed on localhost:6379: %v", err)
	}
	defer rdb.Close()
	fmt.Println("Connected to Redis successfully.")

	// Test Parameters
	const ticketQuota = 100       // Total limited quota
	const concurrentRivals = 2000 // 2,000 concurrent rivals clicking buy at the exact same millisecond
	ticketTypeID := uuid.New().String()
	stockKey := fmt.Sprintf("ticket_stock:%s", ticketTypeID)

	// Pre-warm stock
	if err := rdb.Set(ctx, stockKey, ticketQuota, 0).Err(); err != nil {
		log.Fatalf("Failed to pre-warm stock in Redis: %v", err)
	}
	defer rdb.Del(ctx, stockKey)

	fmt.Printf("Simulating War Tiket: %d limited tickets, %d concurrent requests...\n", ticketQuota, concurrentRivals)

	var successCount int64
	var soldOutCount int64
	var errorCount int64

	startGate := make(chan struct{})
	var wg sync.WaitGroup

	latencies := make([]time.Duration, concurrentRivals)

	for i := 0; i < concurrentRivals; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			<-startGate // Synchronize: start all goroutines simultaneously

			t0 := time.Now()
			res, err := reserveStockLua.Run(ctx, rdb, []string{stockKey}, 1).Int()
			latencies[idx] = time.Since(t0)

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
			} else if res == 1 {
				atomic.AddInt64(&successCount, 1)
			} else if res == 0 {
				atomic.AddInt64(&soldOutCount, 1)
			}
		}()
	}

	testStart := time.Now()
	close(startGate) // Release barrier!
	wg.Wait()
	totalDuration := time.Since(testStart)

	// Calculate latency percentiles
	var totalLatency time.Duration
	for _, l := range latencies {
		totalLatency += l
	}
	avgLatency := totalLatency / time.Duration(concurrentRivals)

	// Verify remaining stock in Redis
	remStock, _ := rdb.Get(ctx, stockKey).Int()

	fmt.Println("\n------------------------------------------------------------------")
	fmt.Println("                        BENCHMARK RESULTS")
	fmt.Println("------------------------------------------------------------------")
	fmt.Printf("Total Concurrent Requests : %d\n", concurrentRivals)
	fmt.Printf("Total Elapsed Time        : %v\n", totalDuration)
	fmt.Printf("Throughput (RPS)          : %.2f req/sec\n", float64(concurrentRivals)/totalDuration.Seconds())
	fmt.Printf("Average Latency per op    : %v\n", avgLatency)
	fmt.Printf("Successful Reservations   : %d (Expected: %d)\n", successCount, ticketQuota)
	fmt.Printf("Sold Out Rejections       : %d (Expected: %d)\n", soldOutCount, concurrentRivals-ticketQuota)
	fmt.Printf("Errors / Timeouts         : %d\n", errorCount)
	fmt.Printf("Final Inventory Stock     : %d (Expected: 0)\n", remStock)

	if successCount == ticketQuota && soldOutCount == int64(concurrentRivals-ticketQuota) && remStock == 0 {
		fmt.Println("\n[PASS] ZERO OVERSELLING GUARANTEED! Concurrency test succeeded perfectly.")
	} else {
		log.Fatalf("\n[FAIL] Race condition / overselling anomaly detected!")
	}

	// Test Rollback
	fmt.Println("\nTesting Rollback Mechanism: Releasing 20 tickets back to stock...")
	_ = releaseStockLua.Run(ctx, rdb, []string{stockKey}, 20).Err()
	reloadedStock, _ := rdb.Get(ctx, stockKey).Int()
	fmt.Printf("Stock after release: %d (Expected: 20)\n", reloadedStock)
	if reloadedStock == 20 {
		fmt.Println("[PASS] Stock Rollback verified successfully!")
	} else {
		log.Fatalf("[FAIL] Rollback verification failed!")
	}
	fmt.Println("==================================================================")
}

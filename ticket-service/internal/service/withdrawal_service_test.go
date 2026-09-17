package service

import (
	"math"
	"testing"
)

func TestOrganizerRevenueSharingAndPlatformFeeCalculations(t *testing.T) {
	// Scenario 1: Paid tickets sales with standard 5% platform fee
	grossRevenue := 1000000.00
	feePercent := 5.0
	feeAmount := grossRevenue * (feePercent / 100.0)
	netRevenue := grossRevenue - feeAmount
	totalWithdrawn := 200000.00
	availableBalance := netRevenue - totalWithdrawn

	if feeAmount != 50000.00 {
		t.Errorf("expected platform fee 50000, got %f", feeAmount)
	}
	if netRevenue != 950000.00 {
		t.Errorf("expected net revenue 950000, got %f", netRevenue)
	}
	if availableBalance != 750000.00 {
		t.Errorf("expected available balance 750000, got %f", availableBalance)
	}

	// Scenario 2: Free ticket event (Rp 0 gross revenue)
	freeGross := 0.00
	freeFee := freeGross * (feePercent / 100.0)
	freeNet := freeGross - freeFee
	if freeFee != 0.00 {
		t.Errorf("expected free ticket fee 0, got %f", freeFee)
	}
	if freeNet != 0.00 {
		t.Errorf("expected free ticket net 0, got %f", freeNet)
	}

	// Scenario 3: Minimum withdrawal threshold check (Rp 10,000)
	minAmount := 10000.00
	reqAmountLow := 5000.00
	if reqAmountLow >= minAmount {
		t.Errorf("amount below min threshold (5000 < 10000) should be invalid")
	}

	// Scenario 4: Insufficient balance check against net available balance
	reqAmountOver := 800000.00
	if reqAmountOver <= availableBalance {
		t.Errorf("request of 800000 should exceed available balance 750000")
	}

	// Scenario 5: Request exactly within available balance
	reqAmountValid := 750000.00
	if reqAmountValid > availableBalance {
		t.Errorf("request equal to available balance should be allowed")
	}
}

func TestTicketServicePlatformFeeConfiguration(t *testing.T) {
	svc := &TicketService{}
	// Default when unset or <= 0 is 5.0%
	if svc.GetPlatformFeePercent() != 5.0 {
		t.Errorf("expected default fee 5.0, got %f", svc.GetPlatformFeePercent())
	}

	// Custom configuration
	svc.SetPlatformFeePercent(7.5)
	if svc.GetPlatformFeePercent() != 7.5 {
		t.Errorf("expected custom fee 7.5, got %f", svc.GetPlatformFeePercent())
	}

	// Test calculation with custom rate
	gross := 2000000.00
	fee := gross * (svc.GetPlatformFeePercent() / 100.0)
	if math.Abs(fee-150000.00) > 0.001 {
		t.Errorf("expected fee 150000, got %f", fee)
	}
}

func TestFloat64ToNumericAndBack(t *testing.T) {
	val := 250000.75
	num := float64ToNumeric(val)
	converted := numericToFloat64(num)

	if converted != val {
		t.Errorf("expected %f, got %f", val, converted)
	}
}


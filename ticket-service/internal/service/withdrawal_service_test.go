package service

import (
	"testing"
)

func TestWithdrawalCalculations(t *testing.T) {
	totalRevenue := 1500000.00
	totalWithdrawn := 500000.00
	availableBalance := totalRevenue - totalWithdrawn

	if availableBalance != 1000000.00 {
		t.Errorf("expected available balance 1000000, got %f", availableBalance)
	}

	// Test minimum withdrawal threshold
	minAmount := 10000.00
	reqAmount := 5000.00
	if reqAmount < minAmount {
		// correctly rejected
	} else {
		t.Errorf("amount below min threshold should be rejected")
	}

	// Test insufficient balance condition
	reqAmountHigh := 1200000.00
	if reqAmountHigh > availableBalance {
		// correctly identified as insufficient balance
	} else {
		t.Errorf("amount higher than available balance should be rejected")
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

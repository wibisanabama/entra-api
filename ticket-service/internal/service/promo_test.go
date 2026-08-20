package service_test

import (
	"strings"
	"testing"
)

func calculateDiscount(code string, subtotal float64) (float64, string, bool) {
	upperCode := strings.ToUpper(strings.TrimSpace(code))
	switch upperCode {
	case "ENTRA20":
		disc := subtotal * 0.20
		if disc > 50000 {
			disc = 50000
		}
		return disc, "Diskon 20% (Maks. Rp 50.000)", true
	case "FESTIVAL50":
		disc := subtotal * 0.50
		if disc > 100000 {
			disc = 100000
		}
		return disc, "Spesial Festival 50% (Maks. Rp 100.000)", true
	case "WELCOME10":
		disc := subtotal * 0.10
		return disc, "Pengguna Baru 10%", true
	case "HEMAT25K":
		if subtotal < 100000 {
			return 0, "Minimal transaksi Rp 100.000 untuk menggunakan voucher ini", false
		}
		return 25000, "Potongan Langsung Rp 25.000", true
	default:
		return 0, "Kode promo tidak valid atau telah kedaluwarsa", false
	}
}

func TestPromoDiscountCalculations(t *testing.T) {
	tests := []struct {
		name          string
		code          string
		subtotal      float64
		wantDiscount  float64
		wantValid     bool
	}{
		{"Valid ENTRA20 under cap", "ENTRA20", 100000, 20000, true},
		{"Valid ENTRA20 with cap limit", "entra20", 500000, 50000, true},
		{"Valid FESTIVAL50 with cap limit", "FESTIVAL50", 400000, 100000, true},
		{"Valid WELCOME10", "WELCOME10", 250000, 25000, true},
		{"HEMAT25K with subtotal meeting threshold", "HEMAT25K", 150000, 25000, true},
		{"HEMAT25K with subtotal below threshold", "HEMAT25K", 80000, 0, false},
		{"Invalid Promo Code", "INVALID99", 200000, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discount, _, valid := calculateDiscount(tt.code, tt.subtotal)
			if valid != tt.wantValid {
				t.Errorf("got valid = %v, want %v", valid, tt.wantValid)
			}
			if discount != tt.wantDiscount {
				t.Errorf("got discount = %v, want %v", discount, tt.wantDiscount)
			}
		})
	}
}

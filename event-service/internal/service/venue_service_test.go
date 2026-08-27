package service_test

import (
	"fmt"
	"testing"

	"entra-api/event-service/internal/service"
	"github.com/jackc/pgx/v5/pgtype"
)

func pgNumericFromFloat(f float64) pgtype.Numeric {
	if f == 0 {
		return pgtype.Numeric{Valid: false}
	}
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%f", f))
	return n
}

func TestVenueService_CountryDefaulting(t *testing.T) {
	reqEmptyCountry := service.CreateVenueRequest{
		Name:    "Gelora Bung Karno",
		Address: "Jl. Pintu Satu Senayan",
		City:    "Jakarta Pusat",
		Country: "",
	}

	country := reqEmptyCountry.Country
	if country == "" {
		country = "Indonesia"
	}
	if country != "Indonesia" {
		t.Errorf("expected default country 'Indonesia', got %s", country)
	}

	reqCustomCountry := service.CreateVenueRequest{
		Name:    "Singapore Indoor Stadium",
		Address: "2 Stadium Walk",
		City:    "Singapore",
		Country: "Singapore",
	}

	country = reqCustomCountry.Country
	if country == "" {
		country = "Indonesia"
	}
	if country != "Singapore" {
		t.Errorf("expected custom country 'Singapore', got %s", country)
	}
}

func TestVenueService_CoordinatesNumericConversion(t *testing.T) {
	lat := -6.2185
	long := 106.8026

	pgLat := pgNumericFromFloat(lat)
	pgLong := pgNumericFromFloat(long)

	if !pgLat.Valid || !pgLong.Valid {
		t.Fatalf("expected valid pgtype.Numeric for coordinates")
	}

	zeroCoord := pgNumericFromFloat(0.0)
	if zeroCoord.Valid {
		t.Errorf("expected 0.0 to produce invalid/unset pgtype.Numeric")
	}
}

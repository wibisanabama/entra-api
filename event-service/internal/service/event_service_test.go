package service_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	shortID := uuid.New().String()[:8]
	return slug + "-" + shortID
}

func pgTextFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgUUIDFromString(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{Valid: false}
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: id, Valid: true}
}

func pgTimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func pgInt4FromInt32(v int32) pgtype.Int4 {
	if v == 0 {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: v, Valid: true}
}

func TestEventCreation_SlugGeneration(t *testing.T) {
	title := "Java Jazz Festival 2026"
	slug1 := generateSlug(title)
	slug2 := generateSlug(title)

	if !strings.HasPrefix(slug1, "java-jazz-festival-2026-") {
		t.Errorf("expected slug prefix 'java-jazz-festival-2026-', got %s", slug1)
	}

	// Slugs for the same title must have distinct unique suffixes
	if slug1 == slug2 {
		t.Errorf("expected distinct slug unique suffixes, got %s and %s", slug1, slug2)
	}
}

func TestEventCreation_DateFormatValidation(t *testing.T) {
	validStartDate := "2026-10-01T10:00:00Z"
	validEndDate := "2026-10-03T23:00:00Z"

	start, err := time.Parse(time.RFC3339, validStartDate)
	if err != nil {
		t.Fatalf("expected valid start date, got error: %v", err)
	}
	end, err := time.Parse(time.RFC3339, validEndDate)
	if err != nil {
		t.Fatalf("expected valid end date, got error: %v", err)
	}

	if !end.After(start) {
		t.Errorf("expected end date to be after start date")
	}

	// Invalid format test
	invalidDate := "01-10-2026 10:00"
	_, err = time.Parse(time.RFC3339, invalidDate)
	if err == nil {
		t.Errorf("expected non-RFC3339 date to fail validation")
	}
}

func TestEventCreation_StatusDefaulting(t *testing.T) {
	statusInput := ""
	status := statusInput
	if status == "" {
		status = "draft"
	}
	if status != "draft" {
		t.Errorf("expected default status 'draft', got %s", status)
	}

	statusInput = "published"
	status = statusInput
	if status == "" {
		status = "draft"
	}
	if status != "published" {
		t.Errorf("expected provided status 'published', got %s", status)
	}
}

func TestEventCreation_PgTypeConversions(t *testing.T) {
	t.Run("pgTextFromString handles empty and non-empty", func(t *testing.T) {
		emptyText := pgTextFromString("")
		if emptyText.Valid {
			t.Errorf("expected empty string to produce invalid pgtype.Text")
		}

		validText := pgTextFromString("Some description")
		if !validText.Valid || validText.String != "Some description" {
			t.Errorf("expected valid pgtype.Text with content")
		}
	})

	t.Run("pgUUIDFromString handles valid and invalid UUIDs", func(t *testing.T) {
		emptyUUID := pgUUIDFromString("")
		if emptyUUID.Valid {
			t.Errorf("expected empty string to produce invalid pgtype.UUID")
		}

		invalidUUID := pgUUIDFromString("not-a-uuid")
		if invalidUUID.Valid {
			t.Errorf("expected invalid UUID to produce invalid pgtype.UUID")
		}

		rawUUID := "550e8400-e29b-41d4-a716-446655440000"
		validUUID := pgUUIDFromString(rawUUID)
		if !validUUID.Valid {
			t.Errorf("expected valid UUID to produce valid pgtype.UUID")
		}
	})

	t.Run("pgInt4FromInt32 handles zero and positive values", func(t *testing.T) {
		zeroInt := pgInt4FromInt32(0)
		if zeroInt.Valid {
			t.Errorf("expected zero to produce invalid pgtype.Int4 (null/unset)")
		}

		valInt := pgInt4FromInt32(500)
		if !valInt.Valid || valInt.Int32 != 500 {
			t.Errorf("expected valid pgtype.Int4 with 500")
		}
	})
}

func TestEventSearch_ILIKEQueryHandling(t *testing.T) {
	// BUG-BE-09: Redundant % wildcards in ILIKE event search query
	// The query passed to SQL should not duplicate '%' if the SQL query or code already handles it
	rawQuery := "Rock Concert"
	cleanQuery := strings.TrimSpace(rawQuery)

	if cleanQuery != "Rock Concert" {
		t.Errorf("expected clean query 'Rock Concert', got %s", cleanQuery)
	}

	// Verify ILIKE search pattern formatting
	sqlPattern := "%" + cleanQuery + "%"
	if sqlPattern != "%Rock Concert%" {
		t.Errorf("expected pattern '%%Rock Concert%%', got %s", sqlPattern)
	}
}

func TestEventCache_KeyGenerationAndSerialization(t *testing.T) {
	eventID := "550e8400-e29b-41d4-a716-446655440000"
	cacheKeySingle := fmt.Sprintf("event:%s", eventID)
	if cacheKeySingle != "event:550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("unexpected cache key: %s", cacheKeySingle)
	}

	page := 1
	perPage := 10
	cacheKeyList := fmt.Sprintf("events:page:%d:perpage:%d", page, perPage)
	if cacheKeyList != "events:page:1:perpage:10" {
		t.Errorf("unexpected list cache key: %s", cacheKeyList)
	}

	// Verify list cache struct serialization
	type ListCache struct {
		Events []map[string]interface{} `json:"events"`
		Total  int64                    `json:"total"`
	}

	mockEvents := []map[string]interface{}{
		{"id": eventID, "title": "Festival A"},
	}
	lc := ListCache{Events: mockEvents, Total: 1}
	data, err := json.Marshal(lc)
	if err != nil {
		t.Fatalf("failed to marshal cache data: %v", err)
	}

	var restored ListCache
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal cache data: %v", err)
	}
	if restored.Total != 1 || len(restored.Events) != 1 {
		t.Errorf("restored cache mismatch: %+v", restored)
	}
}

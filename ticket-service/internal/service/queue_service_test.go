package service

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestQueueDurationAndCalculations(t *testing.T) {
	if QueueActiveDuration != 3*time.Minute {
		t.Errorf("expected QueueActiveDuration to be 3 minutes, got %v", QueueActiveDuration)
	}

	orderID := uuid.New().String()
	eventID := uuid.New().String()

	now := time.Now()
	expiresAt := now.Add(QueueActiveDuration)

	// Test Active Status Response
	respActive := QueueStatusResponse{
		OrderID:          orderID,
		EventID:          eventID,
		Position:         1,
		Status:           "ACTIVE",
		PeopleAhead:      0,
		SecondsRemaining: int64(time.Until(expiresAt).Seconds()),
		ExpiresAt:        expiresAt,
	}

	if respActive.Position != 1 {
		t.Errorf("expected active position to be 1, got %d", respActive.Position)
	}
	if respActive.PeopleAhead != 0 {
		t.Errorf("expected active people ahead to be 0, got %d", respActive.PeopleAhead)
	}
	if respActive.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", respActive.Status)
	}
	if respActive.SecondsRemaining <= 0 || respActive.SecondsRemaining > 180 {
		t.Errorf("expected remaining seconds between 0 and 180, got %d", respActive.SecondsRemaining)
	}

	// Test Waiting Status Response for position 2 and 3
	for pos := 2; pos <= 5; pos++ {
		respWaiting := QueueStatusResponse{
			OrderID:          uuid.New().String(),
			EventID:          eventID,
			Position:         pos,
			Status:           "WAITING",
			PeopleAhead:      pos - 1,
			SecondsRemaining: 0,
		}

		if respWaiting.Position != pos {
			t.Errorf("expected position %d, got %d", pos, respWaiting.Position)
		}
		if respWaiting.PeopleAhead != pos-1 {
			t.Errorf("expected people ahead %d, got %d", pos-1, respWaiting.PeopleAhead)
		}
		if respWaiting.Status != "WAITING" {
			t.Errorf("expected status WAITING, got %s", respWaiting.Status)
		}
	}
}

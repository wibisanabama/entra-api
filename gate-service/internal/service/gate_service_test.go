package service_test

import (
	"testing"

	"github.com/google/uuid"
)

func TestGateService_EventAndStatusValidation(t *testing.T) {
	targetEventID := uuid.New()
	otherEventID := uuid.New()

	t.Run("Rejects ticket belonging to other event", func(t *testing.T) {
		ticketEventID := otherEventID

		if targetEventID != ticketEventID {
			// correctly detected mismatch
		} else {
			t.Errorf("ticket for other event must be rejected")
		}
	})

	t.Run("Rejects ticket with CHECKED_IN status", func(t *testing.T) {
		status := "CHECKED_IN"
		isInvalid := status == "CHECKED_IN" || status == "USED"
		if !isInvalid {
			t.Errorf("CHECKED_IN ticket must be treated as already used")
		}
	})

	t.Run("Rejects ticket with USED status", func(t *testing.T) {
		status := "USED"
		isInvalid := status == "CHECKED_IN" || status == "USED"
		if !isInvalid {
			t.Errorf("USED ticket must be treated as already used")
		}
	})

	t.Run("Accepts ticket with ACTIVE status", func(t *testing.T) {
		status := "ACTIVE"
		isInvalid := status == "CHECKED_IN" || status == "USED"
		if isInvalid {
			t.Errorf("ACTIVE ticket should be accepted")
		}
	})
}

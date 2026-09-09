package service_test

import (
	"testing"

	"github.com/google/uuid"
)

func validateTransferRules(senderID string, recipientID string, ticketStatus string, recipientEmail string) (bool, string) {
	if recipientEmail == "" {
		return false, "email penerima tidak boleh kosong"
	}
	if ticketStatus == "CHECKED_IN" || ticketStatus == "USED" {
		return false, "tiket sudah digunakan dan tidak dapat ditransfer"
	}
	if ticketStatus == "CANCELLED" || ticketStatus == "EXPIRED" {
		return false, "tiket sudah tidak aktif"
	}
	if senderID == recipientID {
		return false, "Anda tidak dapat mentransfer tiket ke akun Anda sendiri"
	}
	return true, ""
}

func TestTransferTicketValidationRules(t *testing.T) {
	senderUUID := uuid.New().String()
	recipientUUID := uuid.New().String()

	tests := []struct {
		name           string
		senderID       string
		recipientID    string
		ticketStatus   string
		recipientEmail string
		wantValid      bool
		wantErrMsg     string
	}{
		{
			name:           "Valid transfer",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "ACTIVE",
			recipientEmail: "friend@example.com",
			wantValid:      true,
			wantErrMsg:     "",
		},
		{
			name:           "Empty recipient email",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "ACTIVE",
			recipientEmail: "",
			wantValid:      false,
			wantErrMsg:     "email penerima tidak boleh kosong",
		},
		{
			name:           "Self transfer rejection",
			senderID:       senderUUID,
			recipientID:    senderUUID,
			ticketStatus:   "ACTIVE",
			recipientEmail: "sender@example.com",
			wantValid:      false,
			wantErrMsg:     "Anda tidak dapat mentransfer tiket ke akun Anda sendiri",
		},
		{
			name:           "Checked in ticket rejection",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "CHECKED_IN",
			recipientEmail: "friend@example.com",
			wantValid:      false,
			wantErrMsg:     "tiket sudah digunakan dan tidak dapat ditransfer",
		},
		{
			name:           "Used ticket rejection",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "USED",
			recipientEmail: "friend@example.com",
			wantValid:      false,
			wantErrMsg:     "tiket sudah digunakan dan tidak dapat ditransfer",
		},
		{
			name:           "Cancelled ticket rejection",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "CANCELLED",
			recipientEmail: "friend@example.com",
			wantValid:      false,
			wantErrMsg:     "tiket sudah tidak aktif",
		},
		{
			name:           "Expired ticket rejection",
			senderID:       senderUUID,
			recipientID:    recipientUUID,
			ticketStatus:   "EXPIRED",
			recipientEmail: "friend@example.com",
			wantValid:      false,
			wantErrMsg:     "tiket sudah tidak aktif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, errMsg := validateTransferRules(tt.senderID, tt.recipientID, tt.ticketStatus, tt.recipientEmail)
			if valid != tt.wantValid {
				t.Errorf("got valid = %v, want %v", valid, tt.wantValid)
			}
			if errMsg != tt.wantErrMsg {
				t.Errorf("got errMsg = %q, want %q", errMsg, tt.wantErrMsg)
			}
		})
	}
}

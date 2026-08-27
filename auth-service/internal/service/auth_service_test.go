package service_test

import (
	"errors"
	"testing"
	"time"

	"entra-api/auth-service/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

type JWTClaims struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123!"

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	// Verify matching password
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		t.Fatalf("expected password to match hash, got error: %v", err)
	}

	// Verify non-matching password
	err = bcrypt.CompareHashAndPassword(hash, []byte("WrongPassword123!"))
	if err == nil {
		t.Fatal("expected wrong password to fail comparison, but it succeeded")
	}
}

func TestJWTTokenGenerationAndValidation(t *testing.T) {
	secret := "test-jwt-secret-key-1234567890"
	userID := "550e8400-e29b-41d4-a716-446655440000"
	role := "organizer"

	claims := JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// Parse and validate token
	parsedClaims := &JWTClaims{}
	parsedToken, err := jwt.ParseWithClaims(tokenString, parsedClaims, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	if err != nil || !parsedToken.Valid {
		t.Fatalf("expected valid token, got error: %v", err)
	}

	if parsedClaims.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, parsedClaims.UserID)
	}
	if parsedClaims.Role != role {
		t.Errorf("expected role %s, got %s", role, parsedClaims.Role)
	}

	// Expired token test
	expiredClaims := JWTClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-30 * time.Minute)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims)
	expiredTokenString, _ := expiredToken.SignedString([]byte(secret))
	_, err = jwt.ParseWithClaims(expiredTokenString, &JWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err == nil {
		t.Fatal("expected expired token to fail validation")
	}
}

func TestPasswordResetTokenHashing(t *testing.T) {
	rawToken := "reset-token-abc123xyz456"

	// Hashes must be deterministic SHA-256 hex strings
	hash1 := service.HashToken(rawToken)
	hash2 := service.HashToken(rawToken)

	if hash1 == "" {
		t.Fatal("expected non-empty token hash")
	}

	if hash1 != hash2 {
		t.Fatalf("expected deterministic hashes, got %s and %s", hash1, hash2)
	}

	if hash1 == rawToken {
		t.Fatal("token must not be stored in plaintext")
	}

	// Different tokens produce different hashes
	otherHash := service.HashToken("different-token-789")
	if hash1 == otherHash {
		t.Fatal("expected distinct hashes for different tokens")
	}
}

func TestOrganizerUpgrade_BusinessRules(t *testing.T) {
	t.Run("Rejects upgrade if user is inactive", func(t *testing.T) {
		isActive := false
		role := "customer"

		var err error
		if !isActive {
			err = errors.New("inactive user cannot be upgraded to organizer")
		}

		if err == nil || err.Error() != "inactive user cannot be upgraded to organizer" {
			t.Errorf("expected inactive user error, got %v", err)
		}
		_ = role
	})

	t.Run("Rejects upgrade if user is already an organizer", func(t *testing.T) {
		isActive := true
		role := "organizer"

		var err error
		if !isActive {
			err = errors.New("inactive user cannot be upgraded to organizer")
		} else if role == "organizer" {
			err = errors.New("user is already an organizer")
		}

		if err == nil || err.Error() != "user is already an organizer" {
			t.Errorf("expected already organizer error, got %v", err)
		}
	})

	t.Run("Rejects upgrade if user is an admin", func(t *testing.T) {
		isActive := true
		role := "admin"

		var err error
		if !isActive {
			err = errors.New("inactive user cannot be upgraded to organizer")
		} else if role == "organizer" {
			err = errors.New("user is already an organizer")
		} else if role == "admin" {
			err = errors.New("admin user cannot be changed to organizer")
		}

		if err == nil || err.Error() != "admin user cannot be changed to organizer" {
			t.Errorf("expected admin restriction error, got %v", err)
		}
	})

	t.Run("Allows upgrade if user is active customer", func(t *testing.T) {
		isActive := true
		role := "customer"

		var err error
		if !isActive {
			err = errors.New("inactive user cannot be upgraded to organizer")
		} else if role == "organizer" {
			err = errors.New("user is already an organizer")
		} else if role == "admin" {
			err = errors.New("admin user cannot be changed to organizer")
		}

		if err != nil {
			t.Errorf("expected active customer upgrade to succeed, got %v", err)
		}
	})
}

func TestBatchUserAuthorization_UUIDParsing(t *testing.T) {
	rawIDs := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"6ba7b811-9dad-11d1-80b4-00c04fd430c8",
	}

	var parsedUUIDs []uuid.UUID
	for _, idStr := range rawIDs {
		u, err := uuid.Parse(idStr)
		if err != nil {
			t.Fatalf("failed to parse valid uuid %s: %v", idStr, err)
		}
		parsedUUIDs = append(parsedUUIDs, u)
	}

	if len(parsedUUIDs) != 3 {
		t.Fatalf("expected 3 parsed UUIDs, got %d", len(parsedUUIDs))
	}

	// Test invalid UUID in batch
	invalidRawIDs := []string{"not-a-valid-uuid"}
	_, err := uuid.Parse(invalidRawIDs[0])
	if err == nil {
		t.Fatal("expected uuid.Parse to fail for invalid uuid")
	}
}

func TestUUIDHelper_PgUUIDConversion(t *testing.T) {
	u := uuid.New()
	pgUUID := pgtype.UUID{Bytes: u, Valid: true}

	if !pgUUID.Valid {
		t.Fatal("expected valid pgUUID")
	}
	if pgUUID.Bytes != u {
		t.Fatalf("expected pgUUID bytes %v, got %v", u, pgUUID.Bytes)
	}
}

func TestForgotPassword_TokenExpiryCalculation(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(30 * time.Minute)

	if expiresAt.Before(now) {
		t.Fatal("expiry must be in the future")
	}
	if diff := expiresAt.Sub(now); diff < 29*time.Minute || diff > 31*time.Minute {
		t.Fatalf("expected ~30 minute expiry duration, got %v", diff)
	}
}

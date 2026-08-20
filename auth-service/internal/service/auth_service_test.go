package service_test

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
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
}

package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"entra-api/auth-service/internal/repository/db"
	"entra-api/shared/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailAlreadyExists = errors.New("email already registered")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// TokenPair holds the access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

// RegisterRequest holds the data needed to register a new user.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required,min=2"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
}

// LoginRequest holds the data needed to login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// RefreshRequest holds the data needed to refresh a token.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ForgotPasswordRequest holds the data for requesting a password reset.
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordRequest holds the data for resetting a password.
type ResetPasswordRequest struct {
	Token       string `json:"token" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// UpdateProfileRequest holds the data needed to update a user's profile.
type UpdateProfileRequest struct {
	FullName  string `json:"full_name" binding:"required,min=2"`
	Phone     string `json:"phone"`
	AvatarURL string `json:"avatar_url"`
}

// AuthService handles authentication business logic.
type AuthService struct {
	queries *db.Queries
	cfg     *config.Config
}

// NewAuthService creates a new AuthService.
func NewAuthService(queries *db.Queries, cfg *config.Config) *AuthService {
	return &AuthService{
		queries: queries,
		cfg:     cfg,
	}
}

// Register creates a new user account.
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*db.User, *TokenPair, error) {
	// Check if email already exists
	_, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err == nil {
		return nil, nil, ErrEmailAlreadyExists
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to check email: %w", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Default role to customer
	role := req.Role
	if role == "" {
		role = "customer"
	}

	// Create user
	user, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		Phone:        pgTextFromString(req.Phone),
		Role:         role,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, &user, "", "")
	if err != nil {
		return nil, nil, err
	}

	return &user, tokens, nil
}

// Login authenticates a user and returns tokens.
func (s *AuthService) Login(ctx context.Context, req LoginRequest, userAgent, ipAddress string) (*db.User, *TokenPair, error) {
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.generateTokenPair(ctx, &user, userAgent, ipAddress)
	if err != nil {
		return nil, nil, err
	}

	return &user, tokens, nil
}

// RefreshToken validates a refresh token and issues new tokens.
func (s *AuthService) RefreshToken(ctx context.Context, req RefreshRequest, userAgent, ipAddress string) (*TokenPair, error) {
	rt, err := s.queries.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	// Delete old refresh token (rotate)
	_ = s.queries.DeleteRefreshToken(ctx, req.RefreshToken)

	user, err := s.queries.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	return s.generateTokenPair(ctx, &user, userAgent, ipAddress)
}

// GetProfile returns the user profile by ID.
func (s *AuthService) GetProfile(ctx context.Context, userID string) (*db.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.queries.GetUserByID(ctx, pgUUIDFromUUID(uid))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

// UpdateProfile updates the user's profile.
func (s *AuthService) UpdateProfile(ctx context.Context, userID string, req UpdateProfileRequest) (*db.User, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	user, err := s.queries.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID:        pgUUIDFromUUID(uid),
		FullName:  req.FullName,
		Phone:     pgTextFromString(req.Phone),
		AvatarUrl: pgTextFromString(req.AvatarURL),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	return &user, nil
}

// UpgradeToOrganizer changes a user's role to organizer.
func (s *AuthService) UpgradeToOrganizer(ctx context.Context, userID string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return ErrUserNotFound
	}

	err = s.queries.UpdateUserRole(ctx, db.UpdateUserRoleParams{
		ID:   pgUUIDFromUUID(uid),
		Role: "organizer",
	})
	if err != nil {
		return fmt.Errorf("failed to upgrade role: %w", err)
	}

	return nil
}

// Logout deletes the refresh token.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	return s.queries.DeleteRefreshToken(ctx, refreshToken)
}

// generateTokenPair creates a new access+refresh token pair.
func (s *AuthService) generateTokenPair(ctx context.Context, user *db.User, userAgent, ipAddress string) (*TokenPair, error) {
	now := time.Now()
	expiresAt := now.Add(s.cfg.JWT.AccessExpiry)

	claims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"role":    user.Role,
		"iat":     now.Unix(),
		"exp":     expiresAt.Unix(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessTokenString, err := accessToken.SignedString([]byte(s.cfg.JWT.Secret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate a secure random refresh token
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}
	refreshTokenString := base64.URLEncoding.EncodeToString(refreshBytes)

	// Store refresh token in DB
	_, err = s.queries.CreateRefreshToken(ctx, db.CreateRefreshTokenParams{
		UserID:    user.ID,
		Token:     refreshTokenString,
		UserAgent: pgTextFromString(userAgent),
		IpAddress: pgTextFromString(ipAddress),
		ExpiresAt: pgTimestamptzFromTime(now.Add(s.cfg.JWT.RefreshExpiry)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    pgTimestamptzFromTime(expiresAt).Time.Unix(),
	}, nil
}

// pgTextFromString converts a Go string to pgtype.Text.
func pgTextFromString(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgUUIDFromUUID(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

func pgTimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// ForgotPassword creates a reset token and returns it.
func (s *AuthService) ForgotPassword(ctx context.Context, email string) (string, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil // don't expose if user exists or not in a real app, but for now we can just return empty
		}
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	// Generate simple token (UUID)
	token := uuid.New().String()
	
	// Set expiry to 30 mins
	expiresAt := time.Now().Add(30 * time.Minute)

	_, err = s.queries.CreatePasswordResetToken(ctx, db.CreatePasswordResetTokenParams{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: pgTimestamptzFromTime(expiresAt),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create reset token: %w", err)
	}

	return token, nil
}

// ResetPassword validates the token and updates the password.
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	resetToken, err := s.queries.GetPasswordResetToken(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("invalid or expired reset token")
		}
		return fmt.Errorf("failed to get reset token: %w", err)
	}

	if resetToken.ExpiresAt.Time.Before(time.Now()) {
		return errors.New("invalid or expired reset token")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	err = s.queries.UpdateUserPassword(ctx, db.UpdateUserPasswordParams{
		ID:           resetToken.UserID,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Delete token
	_ = s.queries.DeletePasswordResetToken(ctx, token)

	return nil
}

// GetUsersByIDs fetches users in batch by their IDs.
func (s *AuthService) GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]db.GetUsersByIDsRow, error) {
	pgIDs := make([]pgtype.UUID, len(ids))
	for i, id := range ids {
		pgIDs[i] = pgUUIDFromUUID(id)
	}
	return s.queries.GetUsersByIDs(ctx, pgIDs)
}

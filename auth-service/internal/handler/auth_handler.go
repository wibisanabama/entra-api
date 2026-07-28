package handler

import (
	"errors"
	"net/http"

	"entra-api/auth-service/internal/repository/db"
	"entra-api/auth-service/internal/service"
	"entra-api/shared/middleware"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	user, tokens, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailAlreadyExists) {
			response.Error(c, http.StatusConflict, err.Error())
			return
		}
		response.InternalError(c, "failed to register user")
		return
	}

	response.Success(c, http.StatusCreated, "user registered successfully", gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// Login handles user login.
func (h *AuthHandler) Login(c *gin.Context) {
	var req service.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	user, tokens, err := h.authService.Login(
		c.Request.Context(),
		req,
		c.Request.UserAgent(),
		c.ClientIP(),
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		response.InternalError(c, "failed to login")
		return
	}

	response.Success(c, http.StatusOK, "login successful", gin.H{
		"user":   sanitizeUser(user),
		"tokens": tokens,
	})
}

// RefreshToken handles token refresh.
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req service.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	tokens, err := h.authService.RefreshToken(
		c.Request.Context(),
		req,
		c.Request.UserAgent(),
		c.ClientIP(),
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) {
			response.Error(c, http.StatusUnauthorized, err.Error())
			return
		}
		response.InternalError(c, "failed to refresh token")
		return
	}

	response.Success(c, http.StatusOK, "token refreshed successfully", gin.H{
		"tokens": tokens,
	})
}

// GetProfile handles fetching the current user's profile.
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get(middleware.AuthUserIDKey)

	user, err := h.authService.GetProfile(c.Request.Context(), userID.(string))
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "failed to get profile")
		return
	}

	response.Success(c, http.StatusOK, "profile retrieved", sanitizeUser(user))
}

// UpdateProfile handles updating the current user's profile.
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID, _ := c.Get(middleware.AuthUserIDKey)

	var req service.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID.(string), req)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.InternalError(c, "failed to update profile")
		return
	}

	response.Success(c, http.StatusOK, "profile updated", sanitizeUser(user))
}

// Logout handles user logout by invalidating the refresh token.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req service.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.InternalError(c, "failed to logout")
		return
	}

	response.Success(c, http.StatusOK, "logged out successfully", nil)
}

func sanitizeUser(user *db.User) gin.H {
	return gin.H{
		"id":          user.ID,
		"email":       user.Email,
		"full_name":   user.FullName,
		"phone":       user.Phone,
		"role":        user.Role,
		"avatar_url":  user.AvatarUrl,
		"is_verified": user.IsVerified,
		"created_at":  user.CreatedAt,
		"updated_at":  user.UpdatedAt,
	}
}

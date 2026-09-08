package handler

import (
	"os"

	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes sets up all routes for the auth service.
func RegisterRoutes(r *gin.Engine, h *AuthHandler, jwtSecret string) {
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "auth-service"})
	})

	api := r.Group("/api/v1")

	// Public auth routes
	auth := api.Group("/auth")
	{
		auth.POST("/register", h.Register)
		auth.POST("/login", h.Login)
		auth.POST("/refresh", h.RefreshToken)
		auth.POST("/forgot-password", h.ForgotPassword)
		auth.POST("/reset-password", h.ResetPassword)
	}

	// Batch users lookup (accessible by internal services or organizers/admins)
	batchAuth := middleware.RequireInternalSecretOrRoles(os.Getenv("INTERNAL_SERVICE_SECRET"), jwtSecret, "organizer", "admin")

	internal := api.Group("/internal")
	internal.Use(batchAuth)
	{
		internal.POST("/users/batch", h.GetUsersBatch)
	}

	authInternal := api.Group("/auth")
	authInternal.Use(batchAuth)
	{
		authInternal.POST("/users/batch", h.GetUsersBatch)
	}

	// Public user routes
	users := api.Group("/users")
	{
		users.GET("/:id", h.GetPublicProfile)
	}

	// Protected auth routes
	protected := api.Group("/auth")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.GET("/profile", h.GetProfile)
		protected.PUT("/profile", h.UpdateProfile)
		protected.POST("/change-password", h.ChangePassword)
		protected.POST("/logout", h.Logout)
		protected.POST("/upgrade", h.UpgradeToOrganizer)
	}
}

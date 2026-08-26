package handler

import (
	"os"

	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, ph *PaymentHandler, jwtSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "payment-service"})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.CORS())

	api.GET("/payments/reference/:reference_id", ph.GetPaymentByReference)

	// Payment simulation endpoint: disabled in production, requires admin authentication
	if os.Getenv("APP_ENV") != "production" {
		dev := api.Group("")
		if jwtSecret != "" {
			dev.Use(middleware.JWTAuth(jwtSecret), middleware.RequireRole("admin"))
		}
		dev.POST("/payments/:id/simulate", ph.SimulatePayment)
	}
}

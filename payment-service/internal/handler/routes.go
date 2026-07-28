package handler

import (
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, ph *PaymentHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "payment-service"})
	})

	api := r.Group("/api/v1")
	api.Use(middleware.CORS()) // simple cors for dev

	// These would normally be protected, but open for simulation
	api.GET("/payments/order/:order_id", ph.GetPaymentByOrder)
	api.POST("/payments/:id/simulate", ph.SimulatePayment)
}

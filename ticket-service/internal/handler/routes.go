package handler

import (
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, oh *OrderHandler, jwtSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ticket-service"})
	})

	api := r.Group("/api/v1")
	protected := api.Group("/tickets")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.POST("/orders", oh.CreateOrder)
	}
}

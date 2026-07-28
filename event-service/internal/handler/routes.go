package handler

import (
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, eh *EventHandler, vh *VenueHandler, ch *CategoryHandler, jwtSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "event-service"})
	})

	api := r.Group("/api/v1")

	// Public routes
	api.GET("/events", eh.List)
	api.GET("/events/:id", eh.Get)
	api.GET("/events/search", eh.Search)
	api.GET("/categories", ch.List)
	api.GET("/venues", vh.List)
	api.GET("/venues/:id", vh.Get)

	// Protected routes (organizer only)
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.POST("/events", eh.Create)
		protected.PUT("/events/:id", eh.Update)
		protected.DELETE("/events/:id", eh.Delete)

		protected.POST("/venues", vh.Create)
		protected.PUT("/venues/:id", vh.Update)
		protected.DELETE("/venues/:id", vh.Delete)
	}
}

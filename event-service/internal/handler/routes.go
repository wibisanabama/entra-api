package handler

import (
	"os"

	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, eh *EventHandler, vh *VenueHandler, ch *CategoryHandler, ith *InternalTicketHandler, tth *TicketTypeHandler, jwtSecret string) {
	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "event-service"})
	})

	api := r.Group("/api/v1")
	
	// Internal routes for ticket-service communication (protected by internal service secret)
	internal := api.Group("/internal")
	internal.Use(middleware.RequireInternalSecret(os.Getenv("INTERNAL_SERVICE_SECRET")))
	{
		internal.POST("/tickets/:id/reserve", ith.ReserveTickets)
		internal.POST("/tickets/:id/release", ith.ReleaseTickets)
		internal.GET("/organizer/:id/events", eh.GetInternalEventIDs)
	}

	// Public routes
	api.GET("/events", eh.List)
	api.GET("/events/:id", eh.Get)
	api.GET("/events/:id/tickets", eh.ListTickets)
	api.GET("/events/search", eh.Search)
	api.GET("/categories", ch.List)
	api.GET("/venues", vh.List)
	api.GET("/venues/:id", vh.Get)

	// Protected routes (organizer or admin only)
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtSecret), middleware.RequireRole("organizer", "admin"))
	{
		protected.GET("/organizer/events", eh.ListByOrganizer)
		protected.POST("/events", eh.Create)
		protected.PUT("/events/:id", eh.Update)
		protected.DELETE("/events/:id", eh.Delete)

		protected.POST("/events/:id/tickets", tth.Create)
		protected.PUT("/events/:id/tickets/:ticket_id", tth.Update)
		protected.DELETE("/events/:id/tickets/:ticket_id", tth.Delete)

		protected.POST("/venues", vh.Create)
		protected.PUT("/venues/:id", vh.Update)
		protected.DELETE("/venues/:id", vh.Delete)
	}
}

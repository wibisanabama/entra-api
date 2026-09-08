package handler

import (
	"os"

	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, oh *OrderHandler, th *TicketHandler, wh *WithdrawalHandler, jwtSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ticket-service"})
	})

	api := r.Group("/api/v1")
	api.POST("/tickets/midtrans/webhook", oh.MidtransWebhook)
	api.POST("/tickets/promo/validate", oh.ValidatePromo)

	// Internal service communication routes
	internal := api.Group("/internal")
	internal.Use(middleware.RequireInternalSecret(os.Getenv("INTERNAL_SERVICE_SECRET")))
	{
		internal.GET("/tickets/code/:code", th.GetTicketByCodeInternal)
		internal.GET("/events/:eventId/gate-stats", th.GetEventGateStatsInternal)
	}

	protected := api.Group("/tickets")
	protected.Use(middleware.JWTAuth(jwtSecret))
	{
		protected.POST("/orders", oh.CreateOrder)
		protected.GET("/orders", oh.ListMyOrders)
		protected.POST("/orders/:id/pay", oh.CreatePaymentToken)
		protected.POST("/orders/:id/simulate", oh.SimulatePayment)
		protected.GET("", th.ListMyTickets)
		protected.GET("/", th.ListMyTickets)
		protected.POST("/:id/transfer", th.TransferTicket)
		protected.GET("/organizer/stats", oh.GetOrganizerStats)
		protected.GET("/organizer/trend", oh.GetSalesTrend)
		protected.GET("/organizer/orders", oh.ListOrganizerOrders)
		protected.GET("/organizer/orders/:id", oh.GetOrganizerOrder)
		protected.GET("/organizer/events/:eventId/attendees", oh.GetEventAttendees)

		// Withdrawal routes for organizers
		protected.GET("/organizer/balance", wh.GetOrganizerBalance)
		protected.POST("/organizer/withdrawals", wh.RequestWithdrawal)
		protected.GET("/organizer/withdrawals", wh.ListOrganizerWithdrawals)
		protected.GET("/organizer/withdrawals/:id", wh.GetOrganizerWithdrawal)

		// Admin withdrawal management routes (Admin only)
		admin := protected.Group("/admin")
		admin.Use(middleware.RequireRole("admin"))
		{
			admin.GET("/withdrawals", wh.AdminListWithdrawals)
			admin.PATCH("/withdrawals/:id/status", wh.AdminUpdateWithdrawalStatus)
		}
	}
}

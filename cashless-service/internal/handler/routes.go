package handler

import (
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, wh *WalletHandler, jwtSecret string) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "cashless-service"})
	})

	api := r.Group("/api/v1/cashless")
	api.Use(middleware.CORS())
	api.Use(middleware.JWTAuth(jwtSecret)) // Require authentication

	api.GET("/wallet", wh.GetWallet)
	api.POST("/topup", wh.TopUp)
	api.POST("/pay", wh.PayAtMerchant)
	api.POST("/refund", wh.RequestRefund)
	api.GET("/transactions", wh.GetTransactions)
}

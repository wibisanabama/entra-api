package handler

import (
	"entra-api/shared/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, gateHandler *GateHandler, jwtSecret string) {
	v1 := router.Group("/api/v1")
	{
		gate := v1.Group("/gate")
		if jwtSecret != "" {
			gate.Use(middleware.JWTAuth(jwtSecret))
		}
		gate.POST("/scan", gateHandler.ScanTicket)
		gate.GET("/stats/:eventId", gateHandler.GetGateStats)
	}
}

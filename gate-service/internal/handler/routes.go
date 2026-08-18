package handler

import (
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(router *gin.Engine, gateHandler *GateHandler) {
	v1 := router.Group("/api/v1")
	{
		gate := v1.Group("/gate")
		gate.POST("/scan", gateHandler.ScanTicket)
		gate.GET("/stats/:eventId", gateHandler.GetGateStats)
	}
}

package handler

import (
	"net/http"

	"entra-api/gate-service/internal/service"
	"entra-api/shared/response"
	"github.com/gin-gonic/gin"
)

type GateHandler struct {
	gateService *service.GateService
}

func NewGateHandler(gateService *service.GateService) *GateHandler {
	return &GateHandler{
		gateService: gateService,
	}
}

type ScanTicketRequest struct {
	TicketCode string `json:"ticket_code" binding:"required"`
}

func (h *GateHandler) ScanTicket(c *gin.Context) {
	var req ScanTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	err := h.gateService.ScanTicket(c.Request.Context(), req.TicketCode)
	if err != nil {
		if err.Error() == "ticket already used or invalid" {
			response.Error(c, http.StatusConflict, err.Error())
		} else {
			response.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Ticket scanned successfully", nil)
}

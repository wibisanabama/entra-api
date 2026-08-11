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
	EventID    string `json:"event_id"`
}

func (h *GateHandler) ScanTicket(c *gin.Context) {
	var req ScanTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "Format kode tiket tidak valid")
		return
	}

	err := h.gateService.ScanTicket(c.Request.Context(), req.TicketCode, req.EventID)
	if err != nil {
		if err.Error() == "ticket already used or invalid" {
			response.Error(c, http.StatusConflict, "Tiket sudah pernah digunakan / di check-in")
		} else if err.Error() == "ticket belongs to another event" {
			response.Error(c, http.StatusForbidden, "Tiket ini milik event lain! Tidak berlaku untuk event ini.")
		} else if err.Error() == "ticket not found" {
			response.Error(c, http.StatusNotFound, "Kode tiket tidak ditemukan pada sistem")
		} else {
			response.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}

	response.Success(c, http.StatusOK, "Check-in berhasil! Tiket valid.", nil)
}

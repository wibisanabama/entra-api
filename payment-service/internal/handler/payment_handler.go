package handler

import (
	"net/http"

	"entra-api/payment-service/internal/repository/db"
	"entra-api/payment-service/internal/service"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type PaymentHandler struct {
	queries        *db.Queries
	paymentService *service.PaymentService
}

func NewPaymentHandler(queries *db.Queries, paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{queries: queries, paymentService: paymentService}
}

func (h *PaymentHandler) GetPaymentByReference(c *gin.Context) {
	refID, err := uuid.Parse(c.Param("reference_id"))
	if err != nil {
		response.ValidationError(c, "invalid reference id")
		return
	}
	refType := c.Param("reference_type")
	if refType == "" {
		refType = "TICKET"
	}

	payment, err := h.queries.GetPaymentByReferenceID(c.Request.Context(), db.GetPaymentByReferenceIDParams{
		ReferenceID:   refID,
		ReferenceType: refType,
	})
	if err != nil {
		response.Error(c, http.StatusNotFound, "payment not found")
		return
	}

	response.Success(c, http.StatusOK, "payment found", payment)
}

type SimulateRequest struct {
	Status string `json:"status" binding:"required,oneof=SUCCESS FAILED"`
}

func (h *PaymentHandler) SimulatePayment(c *gin.Context) {
	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	payment, err := h.paymentService.SimulatePayment(c.Request.Context(), c.Param("id"), req.Status)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "payment simulated", payment)
}

package handler

import (
	"net/http"

	"entra-api/cashless-service/internal/service"
	"entra-api/shared/middleware"
	"entra-api/shared/response"

	"github.com/gin-gonic/gin"
)

type WalletHandler struct {
	walletService *service.WalletService
}

func NewWalletHandler(walletService *service.WalletService) *WalletHandler {
	return &WalletHandler{walletService: walletService}
}

func getUserID(c *gin.Context) string {
	if val := c.GetString(middleware.AuthUserIDKey); val != "" {
		return val
	}
	return c.GetString("user_id")
}

func (h *WalletHandler) GetWallet(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	wallet, err := h.walletService.GetWallet(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "wallet retrieved", wallet)
}

type TopUpRequest struct {
	Amount float64 `json:"amount" binding:"required,min=1000"`
}

func (h *WalletHandler) TopUp(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	var req TopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	res, err := h.walletService.InitiateTopUp(c.Request.Context(), userID, req.Amount)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "topup initiated", res)
}

func (h *WalletHandler) ConfirmTopUp(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	topupID := c.Param("id")
	if topupID == "" {
		response.ValidationError(c, "missing topup id")
		return
	}

	wallet, err := h.walletService.ConfirmTopUp(c.Request.Context(), userID, topupID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Top-up saldo berhasil diselesaikan", wallet)
}

func (h *WalletHandler) MidtransWebhook(c *gin.Context) {
	var payload map[string]interface{}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
		return
	}

	err := h.walletService.HandleMidtransNotification(c.Request.Context(), payload)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "warning": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

type PayRequest struct {
	Amount       float64 `json:"amount" binding:"required,min=1"`
	MerchantID   string  `json:"merchant_id" binding:"required"`
	MerchantName string  `json:"merchant_name"`
}

func (h *WalletHandler) PayAtMerchant(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	var req PayRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	tx, err := h.walletService.PayAtMerchant(c.Request.Context(), userID, req.Amount, req.MerchantID, req.MerchantName)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "payment successful", tx)
}

func (h *WalletHandler) RequestRefund(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	var req service.RefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ValidationError(c, err.Error())
		return
	}

	tx, err := h.walletService.RequestRefund(
		c.Request.Context(),
		userID,
		req.Amount,
		req.BankName,
		req.AccountNumber,
		req.AccountHolder,
		req.Reason,
	)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "Pengajuan refund berhasil diproses.", tx)
}

func (h *WalletHandler) GetTransactions(c *gin.Context) {
	userID := getUserID(c)
	if userID == "" {
		response.Unauthorized(c, "unauthorized: missing user id")
		return
	}

	txs, err := h.walletService.GetTransactions(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, http.StatusOK, "transactions retrieved", txs)
}

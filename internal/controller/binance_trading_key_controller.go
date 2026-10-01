package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/gin-gonic/gin"
)

type BinanceTradingKeyController struct {
	binanceTradingKeyApplication *application.BinanceTradingKeyApplication
}

func NewBinanceTradingKeyController(
	binanceTradingKeyApplication *application.BinanceTradingKeyApplication,
) *BinanceTradingKeyController {
	return &BinanceTradingKeyController{binanceTradingKeyApplication: binanceTradingKeyApplication}
}

// GetTradingKey handles GET /users/me/binance-trading-key; no key answers 200 saying so, since that is the resource's ordinary state.
func (binanceTradingKeyController *BinanceTradingKeyController) GetTradingKey(ginContext *gin.Context) {
	binanceTradingKeyDto, err := binanceTradingKeyController.binanceTradingKeyApplication.GetTradingKey(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		binanceTradingKeyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, binanceTradingKeyDto)
}

// GetTradingKeyStatus handles GET /users/me/binance-trading-key/status, the one view a connector may read.
func (binanceTradingKeyController *BinanceTradingKeyController) GetTradingKeyStatus(ginContext *gin.Context) {
	statusDto, err := binanceTradingKeyController.binanceTradingKeyApplication.GetTradingKeyStatus(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		binanceTradingKeyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, statusDto)
}

// SaveTradingKey handles PUT /users/me/binance-trading-key; PUT because each person has at most one key.
func (binanceTradingKeyController *BinanceTradingKeyController) SaveTradingKey(ginContext *gin.Context) {
	var binanceTradingKeyRequest models.BinanceTradingKeyRequest

	if bindError := ginContext.ShouldBindJSON(&binanceTradingKeyRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	binanceTradingKeyDto, err := binanceTradingKeyController.binanceTradingKeyApplication.SaveTradingKey(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		binanceTradingKeyRequest.ToBinanceTradingKeyWriteDto())
	if err != nil {
		binanceTradingKeyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, binanceTradingKeyDto)
}

// RemoveTradingKey handles DELETE /users/me/binance-trading-key; 204 whether or not a key existed, since the requested state holds either way.
func (binanceTradingKeyController *BinanceTradingKeyController) RemoveTradingKey(ginContext *gin.Context) {
	if err := binanceTradingKeyController.binanceTradingKeyApplication.RemoveTradingKey(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext)); err != nil {
		binanceTradingKeyController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// respondWithError names Binance's refusal in failureReason so callers branch on a value, not on wording.
func (binanceTradingKeyController *BinanceTradingKeyController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrBinanceTradingKeyValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrBinanceTradingKeySealUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	verificationError, isVerificationError := errors.AsType[domains.BinanceTradingKeyVerificationError](err)
	if isVerificationError {
		statusCode := http.StatusUnprocessableEntity
		switch verificationError.Reason {
		case vo.TradingKeyVerificationFailureUnreachable:
			statusCode = http.StatusBadGateway
		case vo.TradingKeyVerificationFailureTimedOut:
			statusCode = http.StatusGatewayTimeout
		}

		ginContext.JSON(statusCode, gin.H{
			"message":       verificationError.Error(),
			"failureReason": string(verificationError.Reason),
		})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

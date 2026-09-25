package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
)

// KCandleContractHistorySyncController fetches a stretch of one contract's history on demand; the operator-set ceiling is the contract venue's own, since venues hold different amounts of history.
type KCandleContractHistorySyncController struct {
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication
	lookbackCeilingDays                 int
}

func NewKCandleContractHistorySyncController(
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication,
	lookbackCeilingDays int,
) *KCandleContractHistorySyncController {
	return &KCandleContractHistorySyncController{
		kCandleContractIngestionApplication: kCandleContractIngestionApplication,
		lookbackCeilingDays:                 lookbackCeilingDays,
	}
}

// StartSymbolHistorySync handles POST /contract-k-candles/history.
func (kCandleContractHistorySyncController *KCandleContractHistorySyncController) StartSymbolHistorySync(
	ginContext *gin.Context,
) {
	var historySyncRequest models.KCandleHistorySyncRequest
	if bindError := ginContext.ShouldBindJSON(&historySyncRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定交易標的與回溯天數"})

		return
	}

	syncRun, syncError := kCandleContractHistorySyncController.kCandleContractIngestionApplication.
		StartSymbolHistorySync(
			ginContext.Request.Context(),
			historySyncRequest.ToSyncDto(),
			kCandleContractHistorySyncController.lookbackCeilingDays)
	// Each case calls for a different caller action: shorten, register or retype, wait for the running sync, or retry later.
	switch {
	case errors.Is(syncError, domains.ErrKCandleHistoryLookback),
		errors.Is(syncError, domains.ErrTradingSymbolNamed):
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": syncError.Error()})

		return
	case errors.Is(syncError, domains.ErrTradingSymbolNotRegistered):
		ginContext.JSON(http.StatusNotFound, gin.H{"message": syncError.Error()})

		return
	case errors.Is(syncError, domains.ErrKCandleHistorySyncInProgress):
		ginContext.JSON(http.StatusConflict, gin.H{"message": syncError.Error()})

		return
	case syncError != nil:
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": syncError.Error()})

		return
	}

	ginContext.JSON(http.StatusAccepted, syncRun)
}

// GetSymbolHistorySync handles GET /contract-k-candles/history/:id.
func (kCandleContractHistorySyncController *KCandleContractHistorySyncController) GetSymbolHistorySync(
	ginContext *gin.Context,
) {
	id, idError := strconv.ParseUint(ginContext.Param("id"), 10, 64)
	if idError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定同步輪次的編號"})

		return
	}

	syncRun, findError := kCandleContractHistorySyncController.kCandleContractIngestionApplication.
		GetSymbolHistorySync(ginContext.Request.Context(), uint(id))
	if errors.Is(findError, service.ErrKCandleHistorySyncRunNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": findError.Error()})

		return
	}
	if findError != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": findError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, syncRun)
}

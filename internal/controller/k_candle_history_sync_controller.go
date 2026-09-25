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

// KCandleHistorySyncController fetches a caller-named stretch of one symbol's history, unlike the catch-up whose reach is system-set; the ceiling is an operator setting.
type KCandleHistorySyncController struct {
	kCandleIngestionApplication *application.KCandleIngestionApplication
	lookbackCeilingDays         int
}

func NewKCandleHistorySyncController(
	kCandleIngestionApplication *application.KCandleIngestionApplication,
	lookbackCeilingDays int,
) *KCandleHistorySyncController {
	return &KCandleHistorySyncController{
		kCandleIngestionApplication: kCandleIngestionApplication,
		lookbackCeilingDays:         lookbackCeilingDays,
	}
}

// StartSymbolHistorySync handles POST /k-candles/history.
// StartSymbolHistorySync handles POST /k-candles/history and answers 202 with the run, since the fetch can take tens of minutes.
func (kCandleHistorySyncController *KCandleHistorySyncController) StartSymbolHistorySync(
	ginContext *gin.Context,
) {
	var historySyncRequest models.KCandleHistorySyncRequest
	if bindError := ginContext.ShouldBindJSON(&historySyncRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest,
			gin.H{"message": "請指定交易標的與回溯天數"})

		return
	}

	syncRun, syncError := kCandleHistorySyncController.kCandleIngestionApplication.StartSymbolHistorySync(
		ginContext.Request.Context(),
		historySyncRequest.ToSyncDto(),
		kCandleHistorySyncController.lookbackCeilingDays)
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

// GetSymbolHistorySync handles GET /k-candles/history/:id.
func (kCandleHistorySyncController *KCandleHistorySyncController) GetSymbolHistorySync(
	ginContext *gin.Context,
) {
	id, idError := strconv.ParseUint(ginContext.Param("id"), 10, 64)
	if idError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定同步輪次的編號"})

		return
	}

	syncRun, findError := kCandleHistorySyncController.kCandleIngestionApplication.GetSymbolHistorySync(
		ginContext.Request.Context(), uint(id))
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

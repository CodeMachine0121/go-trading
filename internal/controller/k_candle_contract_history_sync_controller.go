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

// KCandleContractHistorySyncController fetches a named stretch of one perpetual
// contract's history on demand.
//
// The ceiling is held here rather than by the domain service, because how far this
// system is willing to reach in one request is an operator's decision — and it is the
// contract venue's own ceiling, not the spot one's, because the two venues do not
// hold the same amount of history.
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
//
// **It answers before the fetching is done**, with the run to come back and look at.
// Every minute of this stretch takes two questions to the venue, so a long one is
// twice the round trips the spot side makes and there is no connection worth holding
// open for it.
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
	// The four are deliberately four, because what the caller has to do about them
	// differs: ask for a shorter stretch, register the contract or retype it, wait for
	// the run already going, or come back later.
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

	// Accepted rather than done: the run is recorded and the fetching has started,
	// and the body says where to watch it.
	ginContext.JSON(http.StatusAccepted, syncRun)
}

// GetSymbolHistorySync handles GET /contract-k-candles/history/:id.
//
// The run numbers are the contract venue's own, unrelated to the spot ones, so an
// identifier has to be taken to the listing it came from.
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

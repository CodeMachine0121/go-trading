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

// KCandleHistorySyncController fetches a named stretch of one trading symbol's
// history on demand.
//
// It is its own controller rather than another handler on the catch-up next door,
// because the two differ on the one rule that gives the catch-up its identity: there,
// how far back to reach is the system's own setting and deliberately not the
// caller's. Adding an optional field to that request would make the sentence written
// on it half true, and a half-true rule is harder to keep than none.
//
// The ceiling is held here rather than by the domain service, because how far this
// system is willing to reach in one request is an operator's decision — the same
// reason every other ceiling arrives from the composition root.
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
//
// **It answers before the fetching is done**, with the run to come back and look at.
// Years of one-minute candles is thousands of paced round trips to the source — tens
// of minutes, and longer on a market that answers one day at a time — and no
// connection is worth holding open that long. The work is driven by something that
// outlives this request, so the run is the answer.
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
	// The four are deliberately four, because what the caller has to do about them
	// differs: ask for a shorter stretch, register the symbol or retype it, wait for
	// the run already going, or come back later. A symbol nobody registered in
	// particular must not read the same as storage being down — one says "check what
	// you asked for", the other says "that was fine, try again".
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

// GetSymbolHistorySync handles GET /k-candles/history/:id.
//
// It is what makes the accepted answer above usable: a run identifier with nowhere to
// take it would be a receipt for work nobody can see.
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

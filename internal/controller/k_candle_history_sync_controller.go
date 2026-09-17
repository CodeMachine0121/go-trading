package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
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

// SyncSymbolHistory handles POST /k-candles/history.
//
// It answers only when the whole stretch has been fetched and stored. Ninety days of
// one-minute candles is well over a hundred round trips to the source, so this can
// take minutes — and that is accepted rather than worked around: what calls this is a
// person or a Postman request, not a screen with somebody waiting at it.
func (kCandleHistorySyncController *KCandleHistorySyncController) SyncSymbolHistory(
	ginContext *gin.Context,
) {
	var historySyncRequest models.KCandleHistorySyncRequest
	if bindError := ginContext.ShouldBindJSON(&historySyncRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest,
			gin.H{"message": "請指定交易標的與回溯天數"})

		return
	}

	report, syncError := kCandleHistorySyncController.kCandleIngestionApplication.SyncSymbolHistory(
		ginContext.Request.Context(),
		historySyncRequest.ToSyncDto(),
		kCandleHistorySyncController.lookbackCeilingDays)
	if syncError != nil {
		kCandleHistorySyncController.respondWithError(ginContext, syncError)

		return
	}

	ginContext.JSON(http.StatusOK, report)
}

// respondWithError maps a refusal onto the status code that reports it.
//
// The three are deliberately three, because what the caller has to do about them
// differs: ask for a shorter stretch, register the symbol or retype it, or come back
// later. A symbol nobody registered in particular must not read the same as a source
// that would not answer — one says "check what you asked for", the other says "that
// was fine, try again".
func (kCandleHistorySyncController *KCandleHistorySyncController) respondWithError(
	ginContext *gin.Context, syncError error,
) {
	if errors.Is(syncError, domains.ErrKCandleHistoryLookback) ||
		errors.Is(syncError, domains.ErrTradingSymbolNamed) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": syncError.Error()})

		return
	}

	if errors.Is(syncError, domains.ErrTradingSymbolNotRegistered) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": syncError.Error()})

		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": syncError.Error()})
}

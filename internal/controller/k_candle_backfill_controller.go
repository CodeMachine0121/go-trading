package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// KCandleBackfillController catches one trading symbol up on demand.
//
// It is its own controller rather than another handler on the K candle one, because
// what it exposes is not a candle: it is the ingestion, the same work the start-up
// backfill does, aimed at a single symbol. A caller of this is not reading or writing
// a candle — they are asking the system to go and get some.
type KCandleBackfillController struct {
	kCandleIngestionApplication *application.KCandleIngestionApplication
}

func NewKCandleBackfillController(
	kCandleIngestionApplication *application.KCandleIngestionApplication,
) *KCandleBackfillController {
	return &KCandleBackfillController{kCandleIngestionApplication: kCandleIngestionApplication}
}

// CatchUpSymbol handles POST /k-candles/backfill.
func (kCandleBackfillController *KCandleBackfillController) CatchUpSymbol(ginContext *gin.Context) {
	var backfillRequest models.KCandleBackfillRequest
	if bindError := ginContext.ShouldBindJSON(&backfillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定交易標的"})

		return
	}

	report, catchUpError := kCandleBackfillController.kCandleIngestionApplication.CatchUpSymbol(
		ginContext.Request.Context(), backfillRequest.Symbol)
	if catchUpError != nil {
		// Naming a symbol the system has never been told about is the caller's to fix,
		// and it must not be answered the same way as a source that would not answer —
		// one says "check what you asked for", the other says "come back later".
		if errors.Is(catchUpError, domains.ErrTradingSymbolNotRegistered) {
			ginContext.JSON(http.StatusNotFound, gin.H{"message": catchUpError.Error()})

			return
		}

		if errors.Is(catchUpError, domains.ErrTradingSymbolNamed) {
			ginContext.JSON(http.StatusBadRequest, gin.H{"message": catchUpError.Error()})

			return
		}

		ginContext.JSON(http.StatusBadGateway, gin.H{"message": catchUpError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, report)
}

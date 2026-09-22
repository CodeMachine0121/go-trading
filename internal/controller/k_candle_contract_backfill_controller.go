package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// KCandleContractBackfillController catches one perpetual contract up on demand.
//
// It is its own controller rather than another handler on the contract candle one,
// because what it exposes is not a candle: it is the ingestion, aimed at a single
// contract.
type KCandleContractBackfillController struct {
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication
}

func NewKCandleContractBackfillController(
	kCandleContractIngestionApplication *application.KCandleContractIngestionApplication,
) *KCandleContractBackfillController {
	return &KCandleContractBackfillController{
		kCandleContractIngestionApplication: kCandleContractIngestionApplication,
	}
}

// CatchUpSymbol handles POST /contract-k-candles/backfill.
func (kCandleContractBackfillController *KCandleContractBackfillController) CatchUpSymbol(
	ginContext *gin.Context,
) {
	var backfillRequest models.KCandleBackfillRequest
	if bindError := ginContext.ShouldBindJSON(&backfillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定交易標的"})

		return
	}

	report, catchUpError := kCandleContractBackfillController.kCandleContractIngestionApplication.
		CatchUpSymbol(ginContext.Request.Context(), backfillRequest.Symbol)
	if catchUpError != nil {
		// Naming a contract the system has never been told about is the caller's to
		// fix, and it must not be answered the same way as a source that would not
		// answer — one says "check what you asked for", the other "come back later".
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

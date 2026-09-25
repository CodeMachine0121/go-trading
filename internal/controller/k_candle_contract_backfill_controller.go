package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// KCandleContractBackfillController runs the ingestion for a single perpetual contract on demand.
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
		// An unregistered contract is the caller's to fix and must not read like an unavailable source.
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

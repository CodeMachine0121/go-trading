package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

type TradingSymbolController struct {
	tradingSymbolApplication *application.TradingSymbolApplication
}

func NewTradingSymbolController(
	tradingSymbolApplication *application.TradingSymbolApplication,
) *TradingSymbolController {
	return &TradingSymbolController{tradingSymbolApplication: tradingSymbolApplication}
}

// ListTradingSymbols handles GET /trading-symbols.
func (tradingSymbolController *TradingSymbolController) ListTradingSymbols(ginContext *gin.Context) {
	tradingSymbolDtos, err := tradingSymbolController.tradingSymbolApplication.ListTradingSymbols(ginContext.Request.Context())
	if err != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusOK, tradingSymbolDtos)
}

// AddToWatchlist handles POST /watchlist.
// AddToWatchlist handles POST /watchlist; invalid names and unknown codes are the caller's to fix, while an unreachable market means retry later.
func (tradingSymbolController *TradingSymbolController) AddToWatchlist(ginContext *gin.Context) {
	var watchlistEntryRequest models.WatchlistEntryRequest
	if bindError := ginContext.ShouldBindJSON(&watchlistEntryRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})

		return
	}

	addError := tradingSymbolController.tradingSymbolApplication.AddToWatchlist(
		ginContext.Request.Context(), watchlistEntryRequest.ToDto())
	if addError != nil {
		tradingSymbolController.reportWatchlistFailure(ginContext, addError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

// RemoveFromWatchlist handles DELETE /watchlist/:symbol.
// Removing a symbol that was not watched succeeds, since the requested state holds either way.
func (tradingSymbolController *TradingSymbolController) RemoveFromWatchlist(ginContext *gin.Context) {
	removeError := tradingSymbolController.tradingSymbolApplication.RemoveFromWatchlist(
		ginContext.Request.Context(), ginContext.Param("symbol"))
	if removeError != nil {
		tradingSymbolController.reportWatchlistFailure(ginContext, removeError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

func (tradingSymbolController *TradingSymbolController) reportWatchlistFailure(
	ginContext *gin.Context, watchlistError error,
) {
	isCallersFault := errors.Is(watchlistError, domains.ErrWatchlistEntryValidation) ||
		errors.Is(watchlistError, domains.ErrTradingSymbolNotInMarket)
	if isCallersFault {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": watchlistError.Error()})

		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": watchlistError.Error()})
}

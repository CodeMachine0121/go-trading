package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// TradingSymbolController exposes the trading symbol use cases over HTTP.
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
//
// The three ways this can fail are told apart deliberately, because the reader's next
// move differs for each: a name the system will not accept and a code the market has
// never heard of are both "fix what you typed", while a market that could not be
// reached is "try again shortly" and says nothing about the request at all.
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
//
// Removing something that was not being watched answers the same way as removing
// something that was: what the caller asked for is true either way, and reporting a
// failure would invite them to fix something that is not broken.
func (tradingSymbolController *TradingSymbolController) RemoveFromWatchlist(ginContext *gin.Context) {
	removeError := tradingSymbolController.tradingSymbolApplication.RemoveFromWatchlist(
		ginContext.Request.Context(), ginContext.Param("symbol"))
	if removeError != nil {
		tradingSymbolController.reportWatchlistFailure(ginContext, removeError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

// reportWatchlistFailure maps a watchlist failure onto the status that tells the
// caller what to do about it. Both watchlist handlers reach for it, so the mapping is
// written once and cannot drift between them.
func (tradingSymbolController *TradingSymbolController) reportWatchlistFailure(
	ginContext *gin.Context, watchlistError error,
) {
	isCallersFault := errors.Is(watchlistError, domains.ErrWatchlistEntryValidation) ||
		errors.Is(watchlistError, domains.ErrTradingSymbolNotInMarket)
	if isCallersFault {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": watchlistError.Error()})

		return
	}

	// Everything else — the market unreachable, storage unreachable — is this system
	// failing to do what it was correctly asked to do.
	ginContext.JSON(http.StatusBadGateway, gin.H{"message": watchlistError.Error()})
}

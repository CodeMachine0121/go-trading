package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// ContractTradingSymbolController exposes the perpetual contract watchlist use cases
// over HTTP.
//
// It is its own listing rather than more rows on the spot one, which means a caller
// wanting everything asks twice. That is the price of the spot listing not changing at
// all, and of the same code being followable on both venues at once.
type ContractTradingSymbolController struct {
	contractTradingSymbolApplication *application.ContractTradingSymbolApplication
}

func NewContractTradingSymbolController(
	contractTradingSymbolApplication *application.ContractTradingSymbolApplication,
) *ContractTradingSymbolController {
	return &ContractTradingSymbolController{
		contractTradingSymbolApplication: contractTradingSymbolApplication,
	}
}

// ListContractTradingSymbols handles GET /contract-trading-symbols.
func (contractTradingSymbolController *ContractTradingSymbolController) ListContractTradingSymbols(
	ginContext *gin.Context,
) {
	contractTradingSymbolDtos, findError := contractTradingSymbolController.
		contractTradingSymbolApplication.ListContractTradingSymbols(ginContext.Request.Context())
	if findError != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": findError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, contractTradingSymbolDtos)
}

// AddToWatchlist handles POST /contract-watchlist.
//
// The three ways this can fail are told apart deliberately, because the reader's next
// move differs for each: a name the system will not accept and a code the venue has
// never heard of are both "fix what you typed", while a venue that could not be
// reached is "try again shortly" and says nothing about the request at all.
func (contractTradingSymbolController *ContractTradingSymbolController) AddToWatchlist(
	ginContext *gin.Context,
) {
	var watchlistEntryRequest models.ContractWatchlistEntryRequest
	if bindError := ginContext.ShouldBindJSON(&watchlistEntryRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})

		return
	}

	addError := contractTradingSymbolController.contractTradingSymbolApplication.AddToWatchlist(
		ginContext.Request.Context(), watchlistEntryRequest.Symbol)
	if addError != nil {
		contractTradingSymbolController.reportWatchlistFailure(ginContext, addError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

// RemoveFromWatchlist handles DELETE /contract-watchlist/:symbol.
//
// Removing something that was not being followed answers the same way as removing
// something that was: what the caller asked for is true either way.
func (contractTradingSymbolController *ContractTradingSymbolController) RemoveFromWatchlist(
	ginContext *gin.Context,
) {
	removeError := contractTradingSymbolController.contractTradingSymbolApplication.
		RemoveFromWatchlist(ginContext.Request.Context(), ginContext.Param("symbol"))
	if removeError != nil {
		contractTradingSymbolController.reportWatchlistFailure(ginContext, removeError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

// reportWatchlistFailure maps a watchlist failure onto the status that tells the
// caller what to do about it. Both handlers reach for it, so the mapping is written
// once and cannot drift between them.
func (contractTradingSymbolController *ContractTradingSymbolController) reportWatchlistFailure(
	ginContext *gin.Context, watchlistError error,
) {
	switch {
	case errors.Is(watchlistError, domains.ErrWatchlistEntryValidation),
		errors.Is(watchlistError, domains.ErrTradingSymbolNotInMarket):
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": watchlistError.Error()})
	case errors.Is(watchlistError, domains.ErrMarketDataSourceUnavailable):
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": watchlistError.Error()})
	default:
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": watchlistError.Error()})
	}
}

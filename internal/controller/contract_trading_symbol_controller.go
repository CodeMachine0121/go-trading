package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// ContractTradingSymbolController is a listing separate from the spot one, so the same code can be followed on both venues without changing the spot listing.
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

// AddToWatchlist handles POST /contract-watchlist; invalid names and unknown codes are the caller's to fix, while an unreachable venue means retry later.
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
// Removing a contract that was not followed succeeds, since the requested state holds either way.
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

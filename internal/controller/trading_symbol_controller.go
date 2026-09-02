package controller

import (
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
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
func (tradingSymbolController *TradingSymbolController) ListTradingSymbols(context *gin.Context) {
	tradingSymbolDtos, err := tradingSymbolController.tradingSymbolApplication.ListTradingSymbols()
	if err != nil {
		context.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	context.JSON(http.StatusOK, tradingSymbolDtos)
}

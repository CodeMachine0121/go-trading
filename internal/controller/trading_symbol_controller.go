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
func (tradingSymbolController *TradingSymbolController) ListTradingSymbols(ginContext *gin.Context) {
	tradingSymbolDtos, err := tradingSymbolController.tradingSymbolApplication.ListTradingSymbols(ginContext.Request.Context())
	if err != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusOK, tradingSymbolDtos)
}

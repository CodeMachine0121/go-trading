package application

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingSymbolApplication orchestrates the trading symbol use cases. Each method is
// one call into the domain; no rule or ordering decision lives here.
type TradingSymbolApplication struct {
	tradingSymbolService *service.TradingSymbolService
}

func NewTradingSymbolApplication(tradingSymbolService *service.TradingSymbolService) *TradingSymbolApplication {
	return &TradingSymbolApplication{tradingSymbolService: tradingSymbolService}
}

func (tradingSymbolApplication *TradingSymbolApplication) ListTradingSymbols() ([]dto.TradingSymbolDto, error) {
	return tradingSymbolApplication.tradingSymbolService.ListTradingSymbols()
}

// RegisterDefaultTradingSymbols reports which markets this run newly registered.
func (tradingSymbolApplication *TradingSymbolApplication) RegisterDefaultTradingSymbols() ([]string, error) {
	return tradingSymbolApplication.tradingSymbolService.RegisterDefaultTradingSymbols()
}

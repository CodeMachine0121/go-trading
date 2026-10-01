package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type BinanceTradingKeyApplication struct {
	binanceTradingKeyService *service.BinanceTradingKeyService
}

func NewBinanceTradingKeyApplication(
	binanceTradingKeyService *service.BinanceTradingKeyService,
) *BinanceTradingKeyApplication {
	return &BinanceTradingKeyApplication{binanceTradingKeyService: binanceTradingKeyService}
}

func (binanceTradingKeyApplication *BinanceTradingKeyApplication) GetTradingKey(
	executionContext context.Context, userID uint,
) (dto.BinanceTradingKeyDto, error) {
	return binanceTradingKeyApplication.binanceTradingKeyService.GetTradingKey(executionContext, userID)
}

func (binanceTradingKeyApplication *BinanceTradingKeyApplication) GetTradingKeyStatus(
	executionContext context.Context, userID uint,
) (dto.BinanceTradingKeyStatusDto, error) {
	return binanceTradingKeyApplication.binanceTradingKeyService.GetTradingKeyStatus(
		executionContext, userID)
}

func (binanceTradingKeyApplication *BinanceTradingKeyApplication) SaveTradingKey(
	executionContext context.Context, userID uint, writeDto dto.BinanceTradingKeyWriteDto,
) (dto.BinanceTradingKeyDto, error) {
	return binanceTradingKeyApplication.binanceTradingKeyService.SaveTradingKey(
		executionContext, userID, writeDto)
}

func (binanceTradingKeyApplication *BinanceTradingKeyApplication) RemoveTradingKey(
	executionContext context.Context, userID uint,
) error {
	return binanceTradingKeyApplication.binanceTradingKeyService.RemoveTradingKey(executionContext, userID)
}

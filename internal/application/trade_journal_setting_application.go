package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type TradeJournalSettingApplication struct {
	tradeJournalSettingService *service.TradeJournalSettingService
}

func NewTradeJournalSettingApplication(
	tradeJournalSettingService *service.TradeJournalSettingService,
) *TradeJournalSettingApplication {
	return &TradeJournalSettingApplication{tradeJournalSettingService: tradeJournalSettingService}
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) GetSetting(
	executionContext context.Context, userID uint,
) (dto.TradeJournalSettingDto, error) {
	return tradeJournalSettingApplication.tradeJournalSettingService.GetSetting(executionContext, userID)
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) SaveFeeRates(
	executionContext context.Context, userID uint, writeDto dto.TradeJournalSettingWriteDto,
) (dto.TradeJournalSettingDto, error) {
	return tradeJournalSettingApplication.tradeJournalSettingService.SaveFeeRates(executionContext, userID, writeDto)
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) ListTags(
	executionContext context.Context, ownerID uint,
) ([]dto.TradeTagDto, error) {
	return tradeJournalSettingApplication.tradeJournalSettingService.ListTags(executionContext, ownerID)
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) CreateTag(
	executionContext context.Context, ownerID uint, writeDto dto.TradeTagWriteDto,
) (dto.TradeTagDto, error) {
	return tradeJournalSettingApplication.tradeJournalSettingService.CreateTag(executionContext, ownerID, writeDto)
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) RenameTag(
	executionContext context.Context, ownerID uint, id uint, name string,
) (dto.TradeTagDto, error) {
	return tradeJournalSettingApplication.tradeJournalSettingService.RenameTag(executionContext, ownerID, id, name)
}

func (tradeJournalSettingApplication *TradeJournalSettingApplication) DeleteTag(
	executionContext context.Context, ownerID uint, id uint,
) error {
	return tradeJournalSettingApplication.tradeJournalSettingService.DeleteTag(executionContext, ownerID, id)
}

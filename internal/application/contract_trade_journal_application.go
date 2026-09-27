package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type ContractTradeJournalApplication struct {
	contractTradeJournalService *service.ContractTradeJournalService
	tradeJournalLinkService     *service.TradeJournalLinkService
}

func NewContractTradeJournalApplication(
	contractTradeJournalService *service.ContractTradeJournalService,
	tradeJournalLinkService *service.TradeJournalLinkService,
) *ContractTradeJournalApplication {
	return &ContractTradeJournalApplication{
		contractTradeJournalService: contractTradeJournalService,
		tradeJournalLinkService:     tradeJournalLinkService,
	}
}

// RecordTrade still records a trade whose link round was forgotten since the page opened, only without its source.
func (journalApplication *ContractTradeJournalApplication) RecordTrade(
	executionContext context.Context, viewerID uint, writeDto dto.ContractTradeRecordWriteDto,
) (dto.ContractTradeRecordDto, error) {
	linkRound := journalApplication.tradeJournalLinkService.FindOwnedRoundIfRemembered(
		executionContext, viewerID, writeDto.JournalLinkIdentifier)

	return journalApplication.contractTradeJournalService.RecordTrade(executionContext, viewerID, writeDto, linkRound)
}

func (journalApplication *ContractTradeJournalApplication) AddFill(
	executionContext context.Context, viewerID uint, id uint, fillDto dto.ContractTradeFillWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.AddFill(executionContext, viewerID, id, fillDto)
}

func (journalApplication *ContractTradeJournalApplication) AmendFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint, fillDto dto.ContractTradeFillWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.AmendFill(executionContext, viewerID, id, fillID, fillDto)
}

func (journalApplication *ContractTradeJournalApplication) RemoveFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.RemoveFill(executionContext, viewerID, id, fillID)
}

func (journalApplication *ContractTradeJournalApplication) AmendPlan(
	executionContext context.Context, viewerID uint, id uint, planDto dto.ContractTradePlanWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.AmendPlan(executionContext, viewerID, id, planDto)
}

func (journalApplication *ContractTradeJournalApplication) AddNote(
	executionContext context.Context, viewerID uint, id uint, content string,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.AddNote(executionContext, viewerID, id, content)
}

func (journalApplication *ContractTradeJournalApplication) WriteReview(
	executionContext context.Context, viewerID uint, id uint, reviewDto dto.ContractTradeReviewWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.WriteReview(executionContext, viewerID, id, reviewDto)
}

func (journalApplication *ContractTradeJournalApplication) AssignSetupTags(
	executionContext context.Context, viewerID uint, id uint, setupTagIDs []uint,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.AssignSetupTags(executionContext, viewerID, id, setupTagIDs)
}

func (journalApplication *ContractTradeJournalApplication) DeleteTrade(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return journalApplication.contractTradeJournalService.DeleteTrade(executionContext, viewerID, id)
}

func (journalApplication *ContractTradeJournalApplication) GetTrade(
	executionContext context.Context, viewerID uint, id uint,
) (dto.ContractTradeRecordDto, error) {
	return journalApplication.contractTradeJournalService.GetTrade(executionContext, viewerID, id)
}

func (journalApplication *ContractTradeJournalApplication) ListTrades(
	executionContext context.Context, viewerID uint, queryDto dto.ContractTradeListQueryDto,
) (dto.ContractTradeRecordPageDto, error) {
	return journalApplication.contractTradeJournalService.ListTrades(executionContext, viewerID, queryDto)
}

func (journalApplication *ContractTradeJournalApplication) GetStatistics(
	executionContext context.Context, viewerID uint, period string,
) (dto.ContractTradeStatisticsDto, error) {
	return journalApplication.contractTradeJournalService.GetStatistics(executionContext, viewerID, period)
}

func (journalApplication *ContractTradeJournalApplication) PrepareJournalLink(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (dto.ContractTradePrefillDto, error) {
	linkRound, linkError := journalApplication.tradeJournalLinkService.FindOwnedRound(
		executionContext, viewerID, journalLinkIdentifier)
	if linkError != nil {
		return dto.ContractTradePrefillDto{}, linkError
	}

	return journalApplication.contractTradeJournalService.PrepareJournalLink(executionContext, viewerID, linkRound)
}

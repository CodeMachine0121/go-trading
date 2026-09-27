package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type SpotTradeJournalApplication struct {
	spotTradeJournalService *service.SpotTradeJournalService
	tradeJournalLinkService *service.TradeJournalLinkService
}

func NewSpotTradeJournalApplication(
	spotTradeJournalService *service.SpotTradeJournalService,
	tradeJournalLinkService *service.TradeJournalLinkService,
) *SpotTradeJournalApplication {
	return &SpotTradeJournalApplication{
		spotTradeJournalService: spotTradeJournalService,
		tradeJournalLinkService: tradeJournalLinkService,
	}
}

// RecordTrade still records a trade whose link round was forgotten since the page opened, only without its source.
func (journalApplication *SpotTradeJournalApplication) RecordTrade(
	executionContext context.Context, viewerID uint, writeDto dto.SpotTradeRecordWriteDto,
) (dto.SpotTradeRecordDto, error) {
	linkRound := journalApplication.tradeJournalLinkService.FindOwnedRoundIfRemembered(
		executionContext, viewerID, writeDto.JournalLinkIdentifier)

	return journalApplication.spotTradeJournalService.RecordTrade(executionContext, viewerID, writeDto, linkRound)
}

func (journalApplication *SpotTradeJournalApplication) AddFill(
	executionContext context.Context, viewerID uint, id uint, fillDto dto.SpotTradeFillWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.AddFill(executionContext, viewerID, id, fillDto)
}

func (journalApplication *SpotTradeJournalApplication) AmendFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint, fillDto dto.SpotTradeFillWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.AmendFill(executionContext, viewerID, id, fillID, fillDto)
}

func (journalApplication *SpotTradeJournalApplication) RemoveFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.RemoveFill(executionContext, viewerID, id, fillID)
}

func (journalApplication *SpotTradeJournalApplication) AmendPlan(
	executionContext context.Context, viewerID uint, id uint, planDto dto.SpotTradePlanWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.AmendPlan(executionContext, viewerID, id, planDto)
}

func (journalApplication *SpotTradeJournalApplication) AddNote(
	executionContext context.Context, viewerID uint, id uint, content string,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.AddNote(executionContext, viewerID, id, content)
}

func (journalApplication *SpotTradeJournalApplication) WriteReview(
	executionContext context.Context, viewerID uint, id uint, reviewDto dto.SpotTradeReviewWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.WriteReview(executionContext, viewerID, id, reviewDto)
}

func (journalApplication *SpotTradeJournalApplication) AssignSetupTags(
	executionContext context.Context, viewerID uint, id uint, setupTagIDs []uint,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.AssignSetupTags(executionContext, viewerID, id, setupTagIDs)
}

func (journalApplication *SpotTradeJournalApplication) DeleteTrade(
	executionContext context.Context, viewerID uint, id uint,
) error {
	return journalApplication.spotTradeJournalService.DeleteTrade(executionContext, viewerID, id)
}

func (journalApplication *SpotTradeJournalApplication) GetTrade(
	executionContext context.Context, viewerID uint, id uint,
) (dto.SpotTradeRecordDto, error) {
	return journalApplication.spotTradeJournalService.GetTrade(executionContext, viewerID, id)
}

func (journalApplication *SpotTradeJournalApplication) ListTrades(
	executionContext context.Context, viewerID uint, queryDto dto.SpotTradeListQueryDto,
) (dto.SpotTradeRecordPageDto, error) {
	return journalApplication.spotTradeJournalService.ListTrades(executionContext, viewerID, queryDto)
}

func (journalApplication *SpotTradeJournalApplication) GetStatistics(
	executionContext context.Context, viewerID uint, period string,
) (dto.SpotTradeStatisticsDto, error) {
	return journalApplication.spotTradeJournalService.GetStatistics(executionContext, viewerID, period)
}

func (journalApplication *SpotTradeJournalApplication) PrepareJournalLink(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (dto.SpotTradePrefillDto, error) {
	linkRound, linkError := journalApplication.tradeJournalLinkService.FindOwnedRound(
		executionContext, viewerID, journalLinkIdentifier)
	if linkError != nil {
		return dto.SpotTradePrefillDto{}, linkError
	}

	return journalApplication.spotTradeJournalService.PrepareJournalLink(executionContext, viewerID, linkRound)
}

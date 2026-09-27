package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	contractTradeListDefaultLimit = 20
	contractTradeListMaximumLimit = 200
	// fundingSettlementPageSize matches the repository cap; wider windows are read page by page.
	fundingSettlementPageSize = 1000
)

// ContractTradeJournalService is the application layer's only entry point for the contract trade journal; every trade answers not found to anyone but its owner.
type ContractTradeJournalService struct {
	contractTradeRecordRepository           domaininterface.IContractTradeRecordRepository
	tradeTagRepository                      domaininterface.ITradeTagRepository
	tradeJournalSettingRepository           domaininterface.ITradeJournalSettingRepository
	tradingStrategyRepository               domaininterface.ITradingStrategyRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	strategyBotRepository                   domaininterface.IStrategyBotRepository
	strategyBotRunRecordRepository          domaininterface.IStrategyBotRunRecordRepository
	clockProxy                              domaininterface.IClockProxy
}

func NewContractTradeJournalService(
	contractTradeRecordRepository domaininterface.IContractTradeRecordRepository,
	tradeTagRepository domaininterface.ITradeTagRepository,
	tradeJournalSettingRepository domaininterface.ITradeJournalSettingRepository,
	tradingStrategyRepository domaininterface.ITradingStrategyRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	contractMaintenanceMarginTierRepository domaininterface.IContractMaintenanceMarginTierRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	strategyBotRepository domaininterface.IStrategyBotRepository,
	strategyBotRunRecordRepository domaininterface.IStrategyBotRunRecordRepository,
	clockProxy domaininterface.IClockProxy,
) *ContractTradeJournalService {
	return &ContractTradeJournalService{
		contractTradeRecordRepository:           contractTradeRecordRepository,
		tradeTagRepository:                      tradeTagRepository,
		tradeJournalSettingRepository:           tradeJournalSettingRepository,
		tradingStrategyRepository:               tradingStrategyRepository,
		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository: contractMaintenanceMarginTierRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		kCandleContractRepository:               kCandleContractRepository,
		strategyBotRepository:                   strategyBotRepository,
		strategyBotRunRecordRepository:          strategyBotRunRecordRepository,
		clockProxy:                              clockProxy,
	}
}

func (journalService *ContractTradeJournalService) RecordTrade(
	executionContext context.Context, viewerID uint, writeDto dto.ContractTradeRecordWriteDto,
) (dto.ContractTradeRecordDto, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(writeDto.Symbol))
	if symbolError != nil {
		return dto.ContractTradeRecordDto{}, fmt.Errorf("%w: %w", domains.ErrContractTradeValidation, symbolError)
	}

	_, isRegistered, symbolFindError := journalService.contractTradingSymbolRepository.FindBySymbol(
		executionContext, contractSymbol.Value())
	if symbolFindError != nil {
		return dto.ContractTradeRecordDto{}, symbolFindError
	}
	if !isRegistered {
		return dto.ContractTradeRecordDto{}, fmt.Errorf(
			"%w: 找不到這個合約標的 %s", domains.ErrContractTradeValidation, contractSymbol.Value())
	}

	if writeDto.TradingStrategyID != nil {
		if strategyError := journalService.requireOwnedContractTradingStrategy(
			executionContext, viewerID, *writeDto.TradingStrategyID); strategyError != nil {
			return dto.ContractTradeRecordDto{}, strategyError
		}
	}

	setupTags, tagError := journalService.ownedTags(executionContext, viewerID, writeDto.SetupTagIDs)
	if tagError != nil {
		return dto.ContractTradeRecordDto{}, tagError
	}

	setting, settingError := journalService.settingOf(executionContext, viewerID)
	if settingError != nil {
		return dto.ContractTradeRecordDto{}, settingError
	}

	now := journalService.clockProxy.Now()
	firstEntryFill := setting.PricedFill(writeDto.FirstEntryFill, now)

	recordDomain, validationError := domains.NewOpeningContractTradeRecordDomain(
		viewerID, contractSymbol.Value(), writeDto, firstEntryFill, setupTags, now)
	if validationError != nil {
		return dto.ContractTradeRecordDto{}, validationError
	}

	if writeDto.JournalLinkIdentifier != "" {
		strategyBot, runRecord, linkError := journalService.ownedJournalLinkRound(
			executionContext, viewerID, writeDto.JournalLinkIdentifier)
		// A round forgotten since the page opened still lets the trade be recorded, only without its source.
		if linkError == nil {
			recordDomain.WithSource(strategyBot, runRecord)
		}
	}

	openTrade, hasOpenTrade, openFindError := journalService.contractTradeRecordRepository.
		FindOpenByOwnerSymbolDirection(
			executionContext, viewerID, contractSymbol.Value(), string(recordDomain.Direction()))
	if openFindError != nil {
		return dto.ContractTradeRecordDto{}, openFindError
	}
	if hasOpenTrade {
		return dto.ContractTradeRecordDto{}, domains.ContractTradeOpenPositionExists(
			contractSymbol.Value(), recordDomain.DirectionInWords(), openTrade.ID)
	}

	createdRecord, createError := journalService.contractTradeRecordRepository.Create(
		executionContext, recordDomain.ToEntity())
	if createError != nil {
		return dto.ContractTradeRecordDto{}, createError
	}

	return journalService.detailOf(executionContext, createdRecord), nil
}

func (journalService *ContractTradeJournalService) AddFill(
	executionContext context.Context, viewerID uint, id uint, fillDto dto.ContractTradeFillWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, setting domains.TradeJournalSettingDomain, now time.Time) error {
			return recordDomain.AddFill(setting.PricedFill(fillDto, now), now)
		})
}

func (journalService *ContractTradeJournalService) AmendFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint, fillDto dto.ContractTradeFillWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, setting domains.TradeJournalSettingDomain, now time.Time) error {
			return recordDomain.AmendFill(fillID, setting.PricedFill(fillDto, now), now)
		})
}

func (journalService *ContractTradeJournalService) RemoveFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint,
) (dto.ContractTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, _ domains.TradeJournalSettingDomain, now time.Time) error {
			return recordDomain.RemoveFill(fillID, now)
		})
}

func (journalService *ContractTradeJournalService) AmendPlan(
	executionContext context.Context, viewerID uint, id uint, planDto dto.ContractTradePlanWriteDto,
) (dto.ContractTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, _ domains.TradeJournalSettingDomain, _ time.Time) error {
			return recordDomain.AmendPlan(planDto)
		})
}

func (journalService *ContractTradeJournalService) AddNote(
	executionContext context.Context, viewerID uint, id uint, content string,
) (dto.ContractTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, _ domains.TradeJournalSettingDomain, now time.Time) error {
			return recordDomain.AddNote(content, now)
		})
}

func (journalService *ContractTradeJournalService) WriteReview(
	executionContext context.Context, viewerID uint, id uint, reviewDto dto.ContractTradeReviewWriteDto,
) (dto.ContractTradeRecordDto, error) {
	mistakeTags, tagError := journalService.ownedTags(executionContext, viewerID, reviewDto.MistakeTagIDs)
	if tagError != nil {
		return dto.ContractTradeRecordDto{}, tagError
	}

	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, _ domains.TradeJournalSettingDomain, now time.Time) error {
			return recordDomain.WriteReview(reviewDto, mistakeTags, now)
		})
}

func (journalService *ContractTradeJournalService) AssignSetupTags(
	executionContext context.Context, viewerID uint, id uint, setupTagIDs []uint,
) (dto.ContractTradeRecordDto, error) {
	setupTags, tagError := journalService.ownedTags(executionContext, viewerID, setupTagIDs)
	if tagError != nil {
		return dto.ContractTradeRecordDto{}, tagError
	}

	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.ContractTradeRecordDomain, _ domains.TradeJournalSettingDomain, _ time.Time) error {
			return recordDomain.AssignSetupTags(setupTags)
		})
}

func (journalService *ContractTradeJournalService) DeleteTrade(
	executionContext context.Context, viewerID uint, id uint,
) error {
	if _, findError := journalService.findOwnedTrade(executionContext, viewerID, id); findError != nil {
		return findError
	}

	return journalService.contractTradeRecordRepository.Delete(executionContext, id)
}

func (journalService *ContractTradeJournalService) GetTrade(
	executionContext context.Context, viewerID uint, id uint,
) (dto.ContractTradeRecordDto, error) {
	record, findError := journalService.findOwnedTrade(executionContext, viewerID, id)
	if findError != nil {
		return dto.ContractTradeRecordDto{}, findError
	}

	return journalService.detailOf(executionContext, record), nil
}

// ListTrades works out each trade's money figures but not its excursions or liquidation price, which only the detail needs.
func (journalService *ContractTradeJournalService) ListTrades(
	executionContext context.Context, viewerID uint, queryDto dto.ContractTradeListQueryDto,
) (dto.ContractTradeRecordPageDto, error) {
	filter, filterError := journalService.listFilterOf(queryDto)
	if filterError != nil {
		return dto.ContractTradeRecordPageDto{}, filterError
	}

	records, totalCount, findError := journalService.contractTradeRecordRepository.FindPageByOwner(
		executionContext, viewerID, filter)
	if findError != nil {
		return dto.ContractTradeRecordPageDto{}, findError
	}

	tradingStrategyNames := journalService.tradingStrategyNamesOf(executionContext, viewerID)

	return dto.ContractTradeRecordPageDto{
		Trades:     journalService.summariesOf(executionContext, records, tradingStrategyNames),
		TotalCount: totalCount,
	}, nil
}

// GetStatistics counts trades by when they closed, so a trade opened long ago still belongs to the week it was closed.
func (journalService *ContractTradeJournalService) GetStatistics(
	executionContext context.Context, viewerID uint, period string,
) (dto.ContractTradeStatisticsDto, error) {
	periodDomain, periodError := domains.NewContractTradeStatisticsPeriodDomain(period)
	if periodError != nil {
		return dto.ContractTradeStatisticsDto{}, periodError
	}

	records, findError := journalService.contractTradeRecordRepository.FindClosedByOwner(
		executionContext, viewerID, periodDomain.Since(journalService.clockProxy.Now()))
	if findError != nil {
		return dto.ContractTradeStatisticsDto{}, findError
	}

	closedTrades := journalService.summariesOf(executionContext, records, nil)

	return domains.NewContractTradeStatisticsDomain(periodDomain.Value(), closedTrades).Statistics(), nil
}

// ListComparableGroups groups the person's closed trades that followed a strategy by symbol, each with the replay to set beside it.
func (journalService *ContractTradeJournalService) ListComparableGroups(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) ([]dto.ContractTradeComparisonGroupDto, error) {
	records, findError := journalService.contractTradeRecordRepository.FindClosedByOwnerAndTradingStrategy(
		executionContext, viewerID, tradingStrategyID)
	if findError != nil {
		return nil, findError
	}

	setting, settingError := journalService.settingOf(executionContext, viewerID)
	if settingError != nil {
		return nil, settingError
	}

	closedTrades := journalService.summariesOf(executionContext, records, nil)

	return domains.NewContractTradeLiveComparisonDomain(closedTrades, setting.TakerFeeRate()).Groups(), nil
}

// ComposeLiveComparison lines each group up with how its replay went.
func (journalService *ContractTradeJournalService) ComposeLiveComparison(
	groups []dto.ContractTradeComparisonGroupDto, attempts []dto.ContractTradeBacktestAttemptDto,
) []dto.ContractTradeLiveComparisonRowDto {
	return domains.NewContractTradeLiveComparisonDomain(nil, decimal.Zero).Compose(groups, attempts)
}

// PrepareJournalLink reads what a bot round's link prefills and records nothing; only the bot's owner may read it.
func (journalService *ContractTradeJournalService) PrepareJournalLink(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (dto.ContractTradePrefillDto, error) {
	strategyBot, runRecord, linkError := journalService.ownedJournalLinkRound(
		executionContext, viewerID, journalLinkIdentifier)
	if linkError != nil {
		return dto.ContractTradePrefillDto{}, linkError
	}

	prefillDomain := domains.NewContractTradePrefillDomain(strategyBot, runRecord)
	openTrade, hasOpenTrade, openFindError := journalService.contractTradeRecordRepository.
		FindOpenByOwnerSymbolDirection(executionContext, viewerID, strategyBot.Symbol, prefillDomain.Direction())
	if openFindError != nil {
		return dto.ContractTradePrefillDto{}, openFindError
	}

	return prefillDomain.Prefill(openTrade, hasOpenTrade), nil
}

// ownedJournalLinkRound answers the same not-found for a forgotten round and for somebody else's bot, so links cannot be probed.
func (journalService *ContractTradeJournalService) ownedJournalLinkRound(
	executionContext context.Context, viewerID uint, journalLinkIdentifier string,
) (entities.StrategyBot, entities.StrategyBotRunRecord, error) {
	runRecord, isRemembered, findError := journalService.strategyBotRunRecordRepository.FindByJournalLinkIdentifier(
		executionContext, journalLinkIdentifier)
	if findError != nil {
		return entities.StrategyBot{}, entities.StrategyBotRunRecord{}, findError
	}
	if !isRemembered {
		return entities.StrategyBot{}, entities.StrategyBotRunRecord{}, fmt.Errorf(
			"%w: 這一輪的建議已不在紀錄中", domains.ErrJournalLinkNotFound)
	}

	strategyBot, botError := journalService.strategyBotRepository.FindOne(executionContext, runRecord.StrategyBotID)
	if errors.Is(botError, domains.ErrStrategyBotNotFound) || (botError == nil && strategyBot.OwnerID != viewerID) {
		return entities.StrategyBot{}, entities.StrategyBotRunRecord{}, fmt.Errorf(
			"%w: 找不到這一輪的建議", domains.ErrJournalLinkNotFound)
	}
	if botError != nil {
		return entities.StrategyBot{}, entities.StrategyBotRunRecord{}, botError
	}

	return strategyBot, runRecord, nil
}

// changeTrade is the one path every edit takes: read as the owner, change through the domain, save, and answer with the fresh detail.
func (journalService *ContractTradeJournalService) changeTrade(
	executionContext context.Context, viewerID uint, id uint,
	change func(recordDomain *domains.ContractTradeRecordDomain, setting domains.TradeJournalSettingDomain, now time.Time) error,
) (dto.ContractTradeRecordDto, error) {
	record, findError := journalService.findOwnedTrade(executionContext, viewerID, id)
	if findError != nil {
		return dto.ContractTradeRecordDto{}, findError
	}

	setting, settingError := journalService.settingOf(executionContext, viewerID)
	if settingError != nil {
		return dto.ContractTradeRecordDto{}, settingError
	}

	recordDomain := domains.NewContractTradeRecordDomain(record)
	if changeError := change(&recordDomain, setting, journalService.clockProxy.Now()); changeError != nil {
		return dto.ContractTradeRecordDto{}, changeError
	}

	savedRecord, saveError := journalService.contractTradeRecordRepository.Save(
		executionContext, recordDomain.ToEntity())
	if saveError != nil {
		return dto.ContractTradeRecordDto{}, saveError
	}

	return journalService.detailOf(executionContext, savedRecord), nil
}

func (journalService *ContractTradeJournalService) findOwnedTrade(
	executionContext context.Context, viewerID uint, id uint,
) (entities.ContractTradeRecord, error) {
	record, findError := journalService.contractTradeRecordRepository.FindOne(executionContext, id)
	if findError != nil {
		return entities.ContractTradeRecord{}, findError
	}
	if record.OwnerID != viewerID {
		return entities.ContractTradeRecord{}, domains.ContractTradeNotFound(id)
	}

	return record, nil
}

func (journalService *ContractTradeJournalService) settingOf(
	executionContext context.Context, viewerID uint,
) (domains.TradeJournalSettingDomain, error) {
	setting, _, findError := journalService.tradeJournalSettingRepository.FindOneByUser(executionContext, viewerID)
	if findError != nil {
		return domains.TradeJournalSettingDomain{}, findError
	}

	return domains.NewTradeJournalSettingDomain(setting), nil
}

// ownedTags answers not found for any tag that is missing or somebody else's.
func (journalService *ContractTradeJournalService) ownedTags(
	executionContext context.Context, viewerID uint, tagIDs []uint,
) ([]entities.TradeTag, error) {
	tags, findError := journalService.tradeTagRepository.FindByIDs(executionContext, tagIDs)
	if findError != nil {
		return nil, findError
	}

	tagsByID := map[uint]entities.TradeTag{}
	for _, tag := range tags {
		if tag.OwnerID == viewerID {
			tagsByID[tag.ID] = tag
		}
	}

	ownedTags := make([]entities.TradeTag, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		tag, isOwned := tagsByID[tagID]
		if !isOwned {
			return nil, domains.TradeTagNotFound(tagID)
		}
		ownedTags = append(ownedTags, tag)
	}

	return ownedTags, nil
}

func (journalService *ContractTradeJournalService) requireOwnedContractTradingStrategy(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) error {
	tradingStrategy, findError := journalService.tradingStrategyRepository.FindOne(executionContext, tradingStrategyID)
	if findError != nil {
		return findError
	}
	if tradingStrategy.OwnerID != viewerID {
		return domains.TradingStrategyNotFound(tradingStrategyID)
	}

	marketDataKind, _ := domains.NewMarketDataKindDomain(tradingStrategy.MarketDataKind)
	if !marketDataKind.IsContract() {
		return fmt.Errorf("%w: 只能指名合約交易策略", domains.ErrContractTradeValidation)
	}

	return nil
}

func (journalService *ContractTradeJournalService) listFilterOf(
	queryDto dto.ContractTradeListQueryDto,
) (vo.ContractTradeListFilterVo, error) {
	filter := vo.ContractTradeListFilterVo{Status: queryDto.Status, Limit: queryDto.Limit}

	switch vo.ContractTradeStatusVo(queryDto.Status) {
	case "", vo.ContractTradeStatusOpen, vo.ContractTradeStatusClosed, vo.ContractTradeStatusReviewed:
	default:
		return vo.ContractTradeListFilterVo{}, fmt.Errorf(
			"%w: 狀態只有 open、closed 與 reviewed", domains.ErrContractTradeValidation)
	}

	if strings.TrimSpace(queryDto.Symbol) != "" {
		contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(queryDto.Symbol))
		if symbolError != nil {
			return vo.ContractTradeListFilterVo{}, fmt.Errorf("%w: %w", domains.ErrContractTradeValidation, symbolError)
		}
		filter.Symbol = contractSymbol.Value()
	}

	if queryDto.Period != "" {
		periodDomain, periodError := domains.NewContractTradeStatisticsPeriodDomain(queryDto.Period)
		if periodError != nil {
			return vo.ContractTradeListFilterVo{}, periodError
		}
		filter.OpenedSince = periodDomain.Since(journalService.clockProxy.Now())
	}

	if filter.Limit <= 0 {
		filter.Limit = contractTradeListDefaultLimit
	}
	filter.Limit = min(filter.Limit, contractTradeListMaximumLimit)

	return filter, nil
}

// tradingStrategyNamesOf is nil when the names could not be read, so no trade is wrongly shown as orphaned.
func (journalService *ContractTradeJournalService) tradingStrategyNamesOf(
	executionContext context.Context, viewerID uint,
) map[uint]string {
	tradingStrategies, findError := journalService.tradingStrategyRepository.FindAllByOwner(executionContext, viewerID)
	if findError != nil {
		return nil
	}

	tradingStrategyNames := map[uint]string{}
	for _, tradingStrategy := range tradingStrategies {
		tradingStrategyNames[tradingStrategy.ID] = tradingStrategy.Name
	}

	return tradingStrategyNames
}

// detailOf reads everything one trade's outcome can use; a failed read only leaves its own figure unavailable.
func (journalService *ContractTradeJournalService) detailOf(
	executionContext context.Context, record entities.ContractTradeRecord,
) dto.ContractTradeRecordDto {
	recordDomain := domains.NewContractTradeRecordDomain(record)
	facts := journalService.marketFactsOf(executionContext, []entities.ContractTradeRecord{record})[record.ID]

	holdingEnd := journalService.holdingEndOf(record)
	extremes, extremesError := journalService.kCandleContractRepository.FindPriceExtremesInRange(
		executionContext, record.Symbol, recordDomain.Ledger().FirstEntryAt().Truncate(time.Minute), holdingEnd)
	facts.ExtremesRequested = true
	if extremesError == nil {
		facts.PriceExtremes = extremes
	}

	outcomeDomain := domains.NewContractTradeOutcomeDomain(recordDomain, facts)
	if recordDomain.IsOpen() {
		outcomeDomain = outcomeDomain.WithTradingRules(journalService.tradingRulesOf(executionContext, record.Symbol))
	}

	recordDto := journalService.withLedgerFigures(record.ToDto(), recordDomain)
	recordDto.Outcome = outcomeDomain.Outcome()

	if record.TradingStrategyID != nil {
		tradingStrategy, findError := journalService.tradingStrategyRepository.FindOne(
			executionContext, *record.TradingStrategyID)
		switch {
		case findError == nil:
			recordDto.TradingStrategyName = tradingStrategy.Name
		case errors.Is(findError, domains.ErrTradingStrategyNotFound):
			recordDto.TradingStrategyDeleted = true
		}
	}

	return recordDto
}

func (journalService *ContractTradeJournalService) summariesOf(
	executionContext context.Context, records []entities.ContractTradeRecord, tradingStrategyNames map[uint]string,
) []dto.ContractTradeRecordDto {
	factsByRecord := journalService.marketFactsOf(executionContext, records)

	summaries := make([]dto.ContractTradeRecordDto, 0, len(records))
	for _, record := range records {
		recordDomain := domains.NewContractTradeRecordDomain(record)
		recordDto := journalService.withLedgerFigures(record.ToDto(), recordDomain)
		recordDto.Outcome = domains.NewContractTradeOutcomeDomain(recordDomain, factsByRecord[record.ID]).Outcome()

		if record.TradingStrategyID != nil {
			tradingStrategyName, isKnown := tradingStrategyNames[*record.TradingStrategyID]
			recordDto.TradingStrategyName = tradingStrategyName
			recordDto.TradingStrategyDeleted = tradingStrategyNames != nil && !isKnown
		}

		summaries = append(summaries, recordDto)
	}

	return summaries
}

func (journalService *ContractTradeJournalService) withLedgerFigures(
	recordDto dto.ContractTradeRecordDto, recordDomain domains.ContractTradeRecordDomain,
) dto.ContractTradeRecordDto {
	ledger := recordDomain.Ledger()
	recordDto.AverageEntryPrice = ledger.AverageEntryPrice()
	recordDto.EnteredQuantity = ledger.EnteredQuantity()
	recordDto.Position = ledger.Position()
	if averageExitPrice, hasExited := ledger.AverageExitPrice(); hasExited {
		recordDto.AverageExitPrice.Decimal = averageExitPrice
		recordDto.AverageExitPrice.Valid = true
	}

	return recordDto
}

// marketFactsOf reads each symbol's settlements and latest price once for all its trades, rather than once per trade.
func (journalService *ContractTradeJournalService) marketFactsOf(
	executionContext context.Context, records []entities.ContractTradeRecord,
) map[uint]vo.ContractTradeMarketFactsVo {
	recordsBySymbol := map[string][]entities.ContractTradeRecord{}
	for _, record := range records {
		recordsBySymbol[record.Symbol] = append(recordsBySymbol[record.Symbol], record)
	}

	factsByRecord := map[uint]vo.ContractTradeMarketFactsVo{}
	for symbol, symbolRecords := range recordsBySymbol {
		windowStart := symbolRecords[0].OpenedAt
		windowEnd := journalService.holdingEndOf(symbolRecords[0])
		hasOpenTrade := false
		for _, record := range symbolRecords {
			if record.OpenedAt.Before(windowStart) {
				windowStart = record.OpenedAt
			}
			if holdingEnd := journalService.holdingEndOf(record); holdingEnd.After(windowEnd) {
				windowEnd = holdingEnd
			}
			hasOpenTrade = hasOpenTrade || record.Status == string(vo.ContractTradeStatusOpen)
		}

		contractTradingSymbol, _, _ := journalService.contractTradingSymbolRepository.FindBySymbol(executionContext, symbol)
		schedule := domains.NewFundingSettlementScheduleDomain(contractTradingSymbol)
		settlements := journalService.settlementsBetween(executionContext, symbol, windowStart, windowEnd)

		latestPrice := vo.ContractTradeMarketFactsVo{}
		if hasOpenTrade {
			latestCandles, latestError := journalService.kCandleContractRepository.FindLatest(executionContext, symbol, 1)
			if latestError == nil && len(latestCandles) > 0 {
				latestPrice.LatestPrice = latestCandles[0].Close
				latestPrice.HasLatestPrice = true
			}
		}

		for _, record := range symbolRecords {
			holdingEnd := journalService.holdingEndOf(record)
			facts := latestPrice
			facts.FundingSettlementDue = schedule.IsDueBetween(record.OpenedAt, holdingEnd)
			for _, settlement := range settlements {
				if settlement.SettlementTime.Before(record.OpenedAt) || settlement.SettlementTime.After(holdingEnd) {
					continue
				}
				facts.FundingSettlements = append(facts.FundingSettlements, settlement)
			}
			factsByRecord[record.ID] = facts
		}
	}

	return factsByRecord
}

// settlementsBetween leaves the list empty when reading fails, which the outcome reports as missing data rather than zero.
func (journalService *ContractTradeJournalService) settlementsBetween(
	executionContext context.Context, symbol string, windowStart time.Time, windowEnd time.Time,
) []vo.FundingSettlementVo {
	settlements := []vo.FundingSettlementVo{}
	pageStart := windowStart

	for {
		query, queryError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
			Symbol: symbol, StartTime: pageStart, EndTime: windowEnd,
		})
		if queryError != nil {
			return settlements
		}

		page, findError := journalService.contractFundingRateSettlementRepository.FindInRange(
			executionContext, query, fundingSettlementPageSize)
		if findError != nil {
			return []vo.FundingSettlementVo{}
		}

		for _, settlement := range page {
			settlements = append(settlements, vo.FundingSettlementVo{
				SettlementTime: settlement.SettlementTime.UTC(),
				FundingRate:    settlement.FundingRate,
				MarkPrice:      settlement.MarkPrice,
			})
		}

		if len(page) < fundingSettlementPageSize {
			return settlements
		}
		pageStart = page[len(page)-1].SettlementTime.Add(time.Nanosecond)
	}
}

func (journalService *ContractTradeJournalService) tradingRulesOf(
	executionContext context.Context, symbol string,
) (domains.ContractTradingRulesDomain, bool) {
	contractTradingSymbol, isRegistered, symbolError := journalService.contractTradingSymbolRepository.FindBySymbol(
		executionContext, symbol)
	if symbolError != nil {
		return domains.ContractTradingRulesDomain{}, false
	}

	maintenanceMarginTiers, tierError := journalService.contractMaintenanceMarginTierRepository.FindBySymbol(
		executionContext, symbol)
	if tierError != nil {
		return domains.ContractTradingRulesDomain{}, false
	}

	tradingRules, rulesError := domains.NewContractTradingRulesDomain(
		contractTradingSymbol, isRegistered, maintenanceMarginTiers)

	return tradingRules, rulesError == nil
}

// holdingEndOf is when the trade closed, or now while it is still held.
func (journalService *ContractTradeJournalService) holdingEndOf(record entities.ContractTradeRecord) time.Time {
	if record.ClosedAt != nil {
		return record.ClosedAt.UTC()
	}

	return journalService.clockProxy.Now().UTC()
}

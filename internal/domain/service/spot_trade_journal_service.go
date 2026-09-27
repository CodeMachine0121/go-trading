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
)

const (
	spotTradeListDefaultLimit = 20
	spotTradeListMaximumLimit = 200
)

// SpotTradeJournalService is the application layer's only entry point for the spot trade journal; every trade answers not found to anyone but its owner.
type SpotTradeJournalService struct {
	spotTradeRecordRepository domaininterface.ISpotTradeRecordRepository
	tradeTagRepository        domaininterface.ITradeTagRepository
	tradingStrategyRepository domaininterface.ITradingStrategyRepository
	tradingSymbolRepository   domaininterface.ITradingSymbolRepository
	kCandleRepository         domaininterface.IKCandleRepository
	clockProxy                domaininterface.IClockProxy
}

func NewSpotTradeJournalService(
	spotTradeRecordRepository domaininterface.ISpotTradeRecordRepository,
	tradeTagRepository domaininterface.ITradeTagRepository,
	tradingStrategyRepository domaininterface.ITradingStrategyRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	kCandleRepository domaininterface.IKCandleRepository,
	clockProxy domaininterface.IClockProxy,
) *SpotTradeJournalService {
	return &SpotTradeJournalService{
		spotTradeRecordRepository: spotTradeRecordRepository,
		tradeTagRepository:        tradeTagRepository,
		tradingStrategyRepository: tradingStrategyRepository,
		tradingSymbolRepository:   tradingSymbolRepository,
		kCandleRepository:         kCandleRepository,
		clockProxy:                clockProxy,
	}
}

// RecordTrade copies the round a journal link named onto the trade; a round that is missing or belongs to the contract journal simply leaves no source.
func (journalService *SpotTradeJournalService) RecordTrade(
	executionContext context.Context, viewerID uint, writeDto dto.SpotTradeRecordWriteDto,
	linkRound *dto.JournalLinkRoundDto,
) (dto.SpotTradeRecordDto, error) {
	spotSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(writeDto.Symbol))
	if symbolError != nil {
		return dto.SpotTradeRecordDto{}, fmt.Errorf("%w: %w", domains.ErrSpotTradeValidation, symbolError)
	}

	tradingSymbol, isRegistered, symbolFindError := journalService.tradingSymbolRepository.FindBySymbol(
		executionContext, spotSymbol.Value())
	if symbolFindError != nil {
		return dto.SpotTradeRecordDto{}, symbolFindError
	}
	if !isRegistered {
		return dto.SpotTradeRecordDto{}, fmt.Errorf(
			"%w: 找不到這個現貨標的 %s", domains.ErrSpotTradeValidation, spotSymbol.Value())
	}

	if writeDto.TradingStrategyID != nil {
		tradingStrategy, strategyFindError := journalService.tradingStrategyRepository.FindOne(
			executionContext, *writeDto.TradingStrategyID)
		if strategyFindError != nil {
			return dto.SpotTradeRecordDto{}, strategyFindError
		}
		if tradingStrategy.OwnerID != viewerID {
			return dto.SpotTradeRecordDto{}, domains.TradingStrategyNotFound(*writeDto.TradingStrategyID)
		}
		marketDataKind, _ := domains.NewMarketDataKindDomain(tradingStrategy.MarketDataKind)
		if marketDataKind.IsContract() {
			return dto.SpotTradeRecordDto{}, fmt.Errorf("%w: 只能指名 K 線（現貨）交易策略", domains.ErrSpotTradeValidation)
		}
	}

	setupTags, tagError := journalService.ownedTags(executionContext, viewerID, writeDto.SetupTagIDs)
	if tagError != nil {
		return dto.SpotTradeRecordDto{}, tagError
	}

	now := journalService.clockProxy.Now()
	recordDomain, validationError := domains.NewOpeningSpotTradeRecordDomain(
		viewerID, spotSymbol.Value(), tradingSymbol.Market, writeDto,
		journalService.fillOf(writeDto.FirstBuyFill, now), setupTags, now)
	if validationError != nil {
		return dto.SpotTradeRecordDto{}, validationError
	}

	if linkRound != nil && journalService.belongsToSpotJournal(*linkRound) {
		recordDomain.WithSource(*linkRound)
	}

	openTrade, hasOpenTrade, openFindError := journalService.spotTradeRecordRepository.FindOpenByOwnerSymbol(
		executionContext, viewerID, spotSymbol.Value())
	if openFindError != nil {
		return dto.SpotTradeRecordDto{}, openFindError
	}
	if hasOpenTrade {
		return dto.SpotTradeRecordDto{}, domains.SpotTradeOpenHoldingExists(spotSymbol.Value(), openTrade.ID)
	}

	createdRecord, createError := journalService.spotTradeRecordRepository.Create(
		executionContext, recordDomain.ToEntity())
	if createError != nil {
		return dto.SpotTradeRecordDto{}, createError
	}

	return journalService.detailOf(executionContext, createdRecord), nil
}

func (journalService *SpotTradeJournalService) AddFill(
	executionContext context.Context, viewerID uint, id uint, fillDto dto.SpotTradeFillWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error {
			return recordDomain.AddFill(journalService.fillOf(fillDto, now), now)
		})
}

func (journalService *SpotTradeJournalService) AmendFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint, fillDto dto.SpotTradeFillWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error {
			return recordDomain.AmendFill(fillID, journalService.fillOf(fillDto, now), now)
		})
}

func (journalService *SpotTradeJournalService) RemoveFill(
	executionContext context.Context, viewerID uint, id uint, fillID uint,
) (dto.SpotTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error {
			return recordDomain.RemoveFill(fillID, now)
		})
}

func (journalService *SpotTradeJournalService) AmendPlan(
	executionContext context.Context, viewerID uint, id uint, planDto dto.SpotTradePlanWriteDto,
) (dto.SpotTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, _ time.Time) error {
			return recordDomain.AmendPlan(planDto)
		})
}

func (journalService *SpotTradeJournalService) AddNote(
	executionContext context.Context, viewerID uint, id uint, content string,
) (dto.SpotTradeRecordDto, error) {
	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error {
			return recordDomain.AddNote(content, now)
		})
}

func (journalService *SpotTradeJournalService) WriteReview(
	executionContext context.Context, viewerID uint, id uint, reviewDto dto.SpotTradeReviewWriteDto,
) (dto.SpotTradeRecordDto, error) {
	mistakeTags, tagError := journalService.ownedTags(executionContext, viewerID, reviewDto.MistakeTagIDs)
	if tagError != nil {
		return dto.SpotTradeRecordDto{}, tagError
	}

	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error {
			return recordDomain.WriteReview(reviewDto, mistakeTags, now)
		})
}

func (journalService *SpotTradeJournalService) AssignSetupTags(
	executionContext context.Context, viewerID uint, id uint, setupTagIDs []uint,
) (dto.SpotTradeRecordDto, error) {
	setupTags, tagError := journalService.ownedTags(executionContext, viewerID, setupTagIDs)
	if tagError != nil {
		return dto.SpotTradeRecordDto{}, tagError
	}

	return journalService.changeTrade(executionContext, viewerID, id,
		func(recordDomain *domains.SpotTradeRecordDomain, _ time.Time) error {
			return recordDomain.AssignSetupTags(setupTags)
		})
}

func (journalService *SpotTradeJournalService) DeleteTrade(executionContext context.Context, viewerID uint, id uint) error {
	if _, findError := journalService.findOwnedTrade(executionContext, viewerID, id); findError != nil {
		return findError
	}

	return journalService.spotTradeRecordRepository.Delete(executionContext, id)
}

func (journalService *SpotTradeJournalService) GetTrade(
	executionContext context.Context, viewerID uint, id uint,
) (dto.SpotTradeRecordDto, error) {
	record, findError := journalService.findOwnedTrade(executionContext, viewerID, id)
	if findError != nil {
		return dto.SpotTradeRecordDto{}, findError
	}

	return journalService.detailOf(executionContext, record), nil
}

// ListTrades works out each trade's money figures but not its excursions, which only the detail needs.
func (journalService *SpotTradeJournalService) ListTrades(
	executionContext context.Context, viewerID uint, queryDto dto.SpotTradeListQueryDto,
) (dto.SpotTradeRecordPageDto, error) {
	filter := vo.TradeListFilterVo{Status: queryDto.Status, Limit: queryDto.Limit}

	switch vo.SpotTradeStatusVo(queryDto.Status) {
	case "", vo.SpotTradeStatusOpen, vo.SpotTradeStatusClosed, vo.SpotTradeStatusReviewed:
	default:
		return dto.SpotTradeRecordPageDto{}, fmt.Errorf("%w: 狀態只有 open、closed 與 reviewed", domains.ErrSpotTradeValidation)
	}

	switch vo.MarketVo(queryDto.Market) {
	case "", vo.MarketCrypto, vo.MarketTaiwanStock:
		filter.Market = queryDto.Market
	default:
		return dto.SpotTradeRecordPageDto{}, fmt.Errorf("%w: 市場只有 crypto 與 taiwanStock", domains.ErrSpotTradeValidation)
	}

	if strings.TrimSpace(queryDto.Symbol) != "" {
		spotSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(queryDto.Symbol))
		if symbolError != nil {
			return dto.SpotTradeRecordPageDto{}, fmt.Errorf("%w: %w", domains.ErrSpotTradeValidation, symbolError)
		}
		filter.Symbol = spotSymbol.Value()
	}

	if queryDto.Period != "" {
		periodDomain, periodError := domains.NewTradeStatisticsPeriodDomain(queryDto.Period, domains.ErrSpotTradeValidation)
		if periodError != nil {
			return dto.SpotTradeRecordPageDto{}, periodError
		}
		filter.OpenedSince = periodDomain.Since(journalService.clockProxy.Now())
	}

	if filter.Limit <= 0 {
		filter.Limit = spotTradeListDefaultLimit
	}
	filter.Limit = min(filter.Limit, spotTradeListMaximumLimit)

	records, totalCount, findError := journalService.spotTradeRecordRepository.FindPageByOwner(
		executionContext, viewerID, filter)
	if findError != nil {
		return dto.SpotTradeRecordPageDto{}, findError
	}

	tradingStrategies, strategiesFindError := journalService.tradingStrategyRepository.FindAllByOwner(
		executionContext, viewerID)
	// Unreadable names mark nothing deleted, so no trade is wrongly shown as orphaned.
	tradingStrategyNames := domains.NewTradingStrategyNamesDomain(tradingStrategies, strategiesFindError == nil)

	return dto.SpotTradeRecordPageDto{
		Trades:     journalService.summariesOf(executionContext, records, tradingStrategyNames),
		TotalCount: totalCount,
	}, nil
}

// GetStatistics counts trades by when they closed, one group per market.
func (journalService *SpotTradeJournalService) GetStatistics(
	executionContext context.Context, viewerID uint, period string,
) (dto.SpotTradeStatisticsDto, error) {
	periodDomain, periodError := domains.NewTradeStatisticsPeriodDomain(period, domains.ErrSpotTradeValidation)
	if periodError != nil {
		return dto.SpotTradeStatisticsDto{}, periodError
	}

	records, findError := journalService.spotTradeRecordRepository.FindClosedByOwner(
		executionContext, viewerID, periodDomain.Since(journalService.clockProxy.Now()))
	if findError != nil {
		return dto.SpotTradeStatisticsDto{}, findError
	}

	return domains.NewSpotTradeStatisticsDomain(periodDomain.Value(), journalService.summariesOf(executionContext, records, domains.NewTradingStrategyNamesDomain(nil, false))).
		Statistics(), nil
}

// PlanLiveComparison groups the person's closed spot trades that followed a strategy by symbol, each with the replay to set beside it.
func (journalService *SpotTradeJournalService) PlanLiveComparison(
	executionContext context.Context, viewerID uint, tradingStrategyID uint,
) (dto.SpotTradeComparisonPlanDto, error) {
	records, findError := journalService.spotTradeRecordRepository.FindClosedByOwnerAndTradingStrategy(
		executionContext, viewerID, tradingStrategyID)
	if findError != nil {
		return dto.SpotTradeComparisonPlanDto{}, findError
	}

	return domains.NewSpotTradeLiveComparisonDomain(journalService.summariesOf(executionContext, records, domains.NewTradingStrategyNamesDomain(nil, false))).Plan(), nil
}

// ComposeLiveComparison lines each group up with how its replay went.
func (journalService *SpotTradeJournalService) ComposeLiveComparison(
	groups []dto.SpotTradeComparisonGroupDto, attempts []dto.SpotTradeBacktestAttemptDto,
) []dto.SpotTradeLiveComparisonRowDto {
	return domains.NewSpotTradeLiveComparisonDomain(nil).Compose(groups, attempts)
}

// ComposeLiveComparisonForDeletedTradingStrategy keeps the live figures of a strategy that no longer exists.
func (journalService *SpotTradeJournalService) ComposeLiveComparisonForDeletedTradingStrategy(
	groups []dto.SpotTradeComparisonGroupDto,
) []dto.SpotTradeLiveComparisonRowDto {
	return domains.NewSpotTradeLiveComparisonDomain(nil).ComposeForDeletedTradingStrategy(groups)
}

// PrepareJournalLink reads what a spot bot round's link prefills and records nothing.
func (journalService *SpotTradeJournalService) PrepareJournalLink(
	executionContext context.Context, viewerID uint, linkRound dto.JournalLinkRoundDto,
) (dto.SpotTradePrefillDto, error) {
	if !journalService.belongsToSpotJournal(linkRound) {
		return dto.SpotTradePrefillDto{}, fmt.Errorf("%w: 這條連結屬於合約交易日誌", domains.ErrJournalLinkNotFound)
	}

	tradingSymbol, _, symbolError := journalService.tradingSymbolRepository.FindBySymbol(executionContext, linkRound.Symbol)
	if symbolError != nil {
		return dto.SpotTradePrefillDto{}, symbolError
	}

	openTrade, hasOpenTrade, openFindError := journalService.spotTradeRecordRepository.FindOpenByOwnerSymbol(
		executionContext, viewerID, linkRound.Symbol)
	if openFindError != nil {
		return dto.SpotTradePrefillDto{}, openFindError
	}

	return domains.NewSpotTradePrefillDomain(linkRound, tradingSymbol.Market).Prefill(openTrade, hasOpenTrade), nil
}

func (journalService *SpotTradeJournalService) belongsToSpotJournal(linkRound dto.JournalLinkRoundDto) bool {
	marketDataKind, _ := domains.NewMarketDataKindDomain(linkRound.MarketDataKind)

	return !marketDataKind.IsContract()
}

// changeTrade is the one path every edit takes: read as the owner, change through the domain, save, and answer with the fresh detail.
func (journalService *SpotTradeJournalService) changeTrade(
	executionContext context.Context, viewerID uint, id uint,
	change func(recordDomain *domains.SpotTradeRecordDomain, now time.Time) error,
) (dto.SpotTradeRecordDto, error) {
	record, findError := journalService.findOwnedTrade(executionContext, viewerID, id)
	if findError != nil {
		return dto.SpotTradeRecordDto{}, findError
	}

	recordDomain := domains.NewSpotTradeRecordDomain(record)
	if changeError := change(&recordDomain, journalService.clockProxy.Now()); changeError != nil {
		return dto.SpotTradeRecordDto{}, changeError
	}

	savedRecord, saveError := journalService.spotTradeRecordRepository.Save(executionContext, recordDomain.ToEntity())
	if saveError != nil {
		return dto.SpotTradeRecordDto{}, saveError
	}

	return journalService.detailOf(executionContext, savedRecord), nil
}

// fillOf turns what the person wrote into a buy or sell: a blank time is now and a blank fee is zero, since spot keeps no fee rates.
func (journalService *SpotTradeJournalService) fillOf(fillDto dto.SpotTradeFillWriteDto, now time.Time) entities.SpotTradeFill {
	filledAt := now
	if fillDto.FilledAt != nil {
		filledAt = *fillDto.FilledAt
	}

	return entities.SpotTradeFill{
		Kind:     strings.TrimSpace(fillDto.Kind),
		FilledAt: filledAt.UTC(),
		Price:    fillDto.Price,
		Quantity: fillDto.Quantity,
		Fee:      fillDto.Fee.Decimal,
	}
}

func (journalService *SpotTradeJournalService) findOwnedTrade(
	executionContext context.Context, viewerID uint, id uint,
) (entities.SpotTradeRecord, error) {
	record, findError := journalService.spotTradeRecordRepository.FindOne(executionContext, id)
	if findError != nil {
		return entities.SpotTradeRecord{}, findError
	}
	if record.OwnerID != viewerID {
		return entities.SpotTradeRecord{}, domains.SpotTradeNotFound(id)
	}

	return record, nil
}

// ownedTags answers not found for any tag that is missing or somebody else's; tags are shared by both journals.
func (journalService *SpotTradeJournalService) ownedTags(
	executionContext context.Context, viewerID uint, tagIDs []uint,
) ([]entities.TradeTag, error) {
	foundTags, findError := journalService.tradeTagRepository.FindByIDs(executionContext, tagIDs)
	if findError != nil {
		return nil, findError
	}

	return domains.NewTradeTagSelectionDomain(foundTags, viewerID).Select(tagIDs)
}

// detailOf reads everything one trade's outcome can use; a failed read only leaves its own figure unavailable.
func (journalService *SpotTradeJournalService) detailOf(
	executionContext context.Context, record entities.SpotTradeRecord,
) dto.SpotTradeRecordDto {
	recordDomain := domains.NewSpotTradeRecordDomain(record)
	facts := journalService.latestPricesOf(executionContext, []entities.SpotTradeRecord{record})[record.Symbol]

	extremes, extremesError := journalService.kCandleRepository.FindPriceExtremesInRange(
		executionContext, record.Symbol, recordDomain.Ledger().FirstEntryAt().Truncate(time.Minute),
		journalService.holdingEndOf(record))
	facts.ExtremesRequested = true
	if extremesError == nil {
		facts.PriceExtremes = extremes
	}

	recordDto := journalService.withLedgerFigures(record.ToDto(), recordDomain)
	recordDto.Outcome = domains.NewSpotTradeOutcomeDomain(recordDomain, facts).Outcome()

	if record.TradingStrategyID != nil {
		tradingStrategy, findError := journalService.tradingStrategyRepository.FindOne(
			executionContext, *record.TradingStrategyID)
		followedStrategies := []entities.TradingStrategy{}
		if findError == nil {
			followedStrategies = append(followedStrategies, tradingStrategy)
		}
		recordDto.TradingStrategyName, recordDto.TradingStrategyDeleted = domains.NewTradingStrategyNamesDomain(
			followedStrategies, findError == nil || errors.Is(findError, domains.ErrTradingStrategyNotFound),
		).Describe(record.TradingStrategyID)
	}

	return recordDto
}

func (journalService *SpotTradeJournalService) summariesOf(
	executionContext context.Context, records []entities.SpotTradeRecord, tradingStrategyNames domains.TradingStrategyNamesDomain,
) []dto.SpotTradeRecordDto {
	latestPrices := journalService.latestPricesOf(executionContext, records)

	summaries := make([]dto.SpotTradeRecordDto, 0, len(records))
	for _, record := range records {
		recordDomain := domains.NewSpotTradeRecordDomain(record)
		recordDto := journalService.withLedgerFigures(record.ToDto(), recordDomain)
		recordDto.Outcome = domains.NewSpotTradeOutcomeDomain(recordDomain, latestPrices[record.Symbol]).Outcome()

		recordDto.TradingStrategyName, recordDto.TradingStrategyDeleted = tradingStrategyNames.Describe(record.TradingStrategyID)

		summaries = append(summaries, recordDto)
	}

	return summaries
}

func (journalService *SpotTradeJournalService) withLedgerFigures(
	recordDto dto.SpotTradeRecordDto, recordDomain domains.SpotTradeRecordDomain,
) dto.SpotTradeRecordDto {
	ledger := recordDomain.Ledger()
	recordDto.Currency = recordDomain.MarketDomain().Currency()
	recordDto.AverageBuyPrice = ledger.AverageEntryPrice()
	recordDto.BoughtQuantity = ledger.EnteredQuantity()
	recordDto.Holding = ledger.Position()
	if averageSellPrice, hasSold := ledger.AverageExitPrice(); hasSold {
		recordDto.AverageSellPrice.Decimal = averageSellPrice
		recordDto.AverageSellPrice.Valid = true
	}

	return recordDto
}

// latestPricesOf reads each held symbol's latest price once for all its trades, rather than once per trade.
func (journalService *SpotTradeJournalService) latestPricesOf(
	executionContext context.Context, records []entities.SpotTradeRecord,
) map[string]vo.SpotTradeMarketFactsVo {
	factsBySymbol := map[string]vo.SpotTradeMarketFactsVo{}
	for _, record := range records {
		if _, isRead := factsBySymbol[record.Symbol]; isRead || record.Status != string(vo.SpotTradeStatusOpen) {
			continue
		}

		facts := vo.SpotTradeMarketFactsVo{}
		latestCandles, latestError := journalService.kCandleRepository.FindLatest(executionContext, record.Symbol, 1)
		if latestError == nil && len(latestCandles) > 0 {
			facts.LatestPrice = latestCandles[0].Close
			facts.HasLatestPrice = true
		}
		factsBySymbol[record.Symbol] = facts
	}

	return factsBySymbol
}

// holdingEndOf is when the trade closed, or now while it is still held.
func (journalService *SpotTradeJournalService) holdingEndOf(record entities.SpotTradeRecord) time.Time {
	if record.ClosedAt != nil {
		return record.ClosedAt.UTC()
	}

	return journalService.clockProxy.Now().UTC()
}

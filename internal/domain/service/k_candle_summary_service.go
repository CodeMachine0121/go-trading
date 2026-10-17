package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// recentSummaryWindow is how far back each round looks: the last two finished days, so yesterday is summarized again after late corrections to its final minutes voided it.
const recentSummaryWindow = 48 * time.Hour

// KCandleSummaryService fills the recently finished days of every watched spot symbol and contract, so viewers of those rarely wait for a day to be merged; older gaps are filled when someone looks at them.
type KCandleSummaryService struct {
	summarizedSeriesReader          kCandleSummarizedSeriesReader
	contractSummarizedSeriesReader  kCandleContractSummarizedSeriesReader
	tradingSymbolRepository         domaininterface.ITradingSymbolRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	clockProxy                      domaininterface.IClockProxy
	marketCatalogDomain             domains.MarketCatalogDomain
}

func NewKCandleSummaryService(
	kCandleRepository domaininterface.IKCandleRepository,
	kCandleSummaryDayRepository domaininterface.IKCandleSummaryDayRepository,
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	kCandleContractSummaryDayRepository domaininterface.IKCandleContractSummaryDayRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
) *KCandleSummaryService {
	return &KCandleSummaryService{
		summarizedSeriesReader: newKCandleSummarizedSeriesReader(
			kCandleRepository, kCandleSummaryDayRepository),
		contractSummarizedSeriesReader: newKCandleContractSummarizedSeriesReader(
			kCandleContractRepository, kCandleContractSummaryDayRepository),
		tradingSymbolRepository:         tradingSymbolRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		clockProxy:                      clockProxy,
		marketCatalogDomain:             marketCatalogDomain,
	}
}

// SummarizeRecentDays keeps going past a symbol that fails, so one unreadable symbol does not starve the rest, and reports every failure together.
func (kCandleSummaryService *KCandleSummaryService) SummarizeRecentDays(executionContext context.Context) error {
	todayStart := kCandleSummaryService.clockProxy.Now().UTC().Truncate(domains.KCandleSummaryDayLength)
	recentDays := dto.KCandleSeriesQueryDto{
		StartTime: todayStart.Add(-recentSummaryWindow),
		EndTime:   todayStart,
		Interval:  string(vo.AggregationIntervalOneDay),
	}
	recentDayCount := int(recentSummaryWindow/domains.KCandleSummaryDayLength) + 1

	watchedSymbols, findError := kCandleSummaryService.tradingSymbolRepository.FindWatched(executionContext)
	if findError != nil {
		return findError
	}
	watchedContracts, findContractsError := kCandleSummaryService.contractTradingSymbolRepository.
		FindWatched(executionContext)
	if findContractsError != nil {
		return findContractsError
	}

	symbolErrors := make([]error, 0)
	for _, watchedSymbol := range watchedSymbols {
		recentDays.Symbol = watchedSymbol.Symbol
		seriesQueryDomain, summarizeError := domains.NewKCandleSeriesQueryDomain(recentDays,
			kCandleSummaryService.marketCatalogDomain.MarketOf(watchedSymbol.Market), recentDayCount)
		if summarizeError == nil {
			_, summarizeError = kCandleSummaryService.summarizedSeriesReader.summaries(executionContext,
				domains.NewKCandleSummaryPlanDomain(seriesQueryDomain, todayStart))
		}
		if summarizeError != nil {
			symbolErrors = append(symbolErrors, fmt.Errorf("summarize %s: %w", watchedSymbol.Symbol, summarizeError))
		}
	}
	for _, watchedContract := range watchedContracts {
		recentDays.Symbol = watchedContract.Symbol
		seriesQueryDomain, summarizeError := domains.NewKCandleSeriesQueryDomain(recentDays,
			kCandleSummaryService.marketCatalogDomain.MarketOf(string(vo.MarketCrypto)), recentDayCount)
		if summarizeError == nil {
			_, summarizeError = kCandleSummaryService.contractSummarizedSeriesReader.summaries(executionContext,
				domains.NewKCandleSummaryPlanDomain(seriesQueryDomain, todayStart))
		}
		if summarizeError != nil {
			symbolErrors = append(symbolErrors,
				fmt.Errorf("summarize contract %s: %w", watchedContract.Symbol, summarizeError))
		}
	}

	return errors.Join(symbolErrors...)
}

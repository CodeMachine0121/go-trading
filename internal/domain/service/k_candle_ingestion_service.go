package service

import (
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleIngestionService keeps the stored K candles current without anyone asking.
// Its two public use cases never call one another: the backfill closes the gap left
// while nothing was running, the periodic round keeps up with the market afterwards.
//
// Both funnel into one path, because they differ only in the stretch of time they
// ask the source for. Whatever the source answers is judged by the ordinary K candle
// rules, one candle at a time.
type KCandleIngestionService struct {
	kCandleRepository domaininterface.IKCandleRepository
	marketDataProxy   domaininterface.IMarketDataProxy
	clockProxy        domaininterface.IClockProxy
	roundCandleCount  int
	backfillLookback  time.Duration
}

func NewKCandleIngestionService(
	kCandleRepository domaininterface.IKCandleRepository,
	marketDataProxy domaininterface.IMarketDataProxy,
	clockProxy domaininterface.IClockProxy,
	roundCandleCount int,
	backfillLookback time.Duration,
) *KCandleIngestionService {
	return &KCandleIngestionService{
		kCandleRepository: kCandleRepository,
		marketDataProxy:   marketDataProxy,
		clockProxy:        clockProxy,
		roundCandleCount:  roundCandleCount,
		backfillLookback:  backfillLookback,
	}
}

// RunScheduledRound fetches the newest closed candles for every watched symbol and
// stores them, replacing whatever was held for the same open time. It reports what
// happened rather than failing: one symbol the source would not answer for must not
// take the others down with it.
func (kCandleIngestionService *KCandleIngestionService) RunScheduledRound(
	symbols []string,
) (dto.KCandleIngestionReportDto, error) {
	ingestionDomain, buildError := kCandleIngestionService.newIngestionDomain()
	if buildError != nil {
		return dto.KCandleIngestionReportDto{}, buildError
	}

	return kCandleIngestionService.ingestSymbols(ingestionDomain, symbols,
		func(symbol string) (vo.KCandleFetchWindowVo, error) {
			return ingestionDomain.ScheduledWindow(symbol), nil
		}), nil
}

// RunBackfill closes each watched symbol's gap, reaching no further back than the
// lookback allows. A symbol already up to date is left alone and its source is never
// called.
func (kCandleIngestionService *KCandleIngestionService) RunBackfill(
	symbols []string,
) (dto.KCandleIngestionReportDto, error) {
	ingestionDomain, buildError := kCandleIngestionService.newIngestionDomain()
	if buildError != nil {
		return dto.KCandleIngestionReportDto{}, buildError
	}

	return kCandleIngestionService.ingestSymbols(ingestionDomain, symbols,
		func(symbol string) (vo.KCandleFetchWindowVo, error) {
			latestStored, findError := kCandleIngestionService.kCandleRepository.FindLatest(symbol, 1)
			if findError != nil {
				return vo.KCandleFetchWindowVo{}, findError
			}

			if len(latestStored) == 0 {
				return ingestionDomain.BackfillWindow(symbol, time.Time{}), nil
			}

			return ingestionDomain.BackfillWindow(symbol, latestStored[0].OpenTime), nil
		}), nil
}

// newIngestionDomain reads the clock once, so that every window in one run and every
// candle judged during it share the same idea of "now".
func (kCandleIngestionService *KCandleIngestionService) newIngestionDomain() (domains.KCandleIngestionDomain, error) {
	return domains.NewKCandleIngestionDomain(
		kCandleIngestionService.clockProxy.Now(),
		kCandleIngestionService.roundCandleCount,
		kCandleIngestionService.backfillLookback,
	)
}

// ingestSymbols runs every watched symbol at once. A plain wait group is deliberate:
// an error group would cancel the remaining symbols the moment one failed, which is
// the opposite of what independence per symbol means here. Each goroutine owns one
// slot of the result, so nothing is shared and nothing needs locking.
func (kCandleIngestionService *KCandleIngestionService) ingestSymbols(
	ingestionDomain domains.KCandleIngestionDomain,
	symbols []string,
	windowOf func(symbol string) (vo.KCandleFetchWindowVo, error),
) dto.KCandleIngestionReportDto {
	symbolReports := make([]dto.KCandleSymbolIngestionReportDto, len(symbols))

	var waitGroup sync.WaitGroup
	for index, symbol := range symbols {
		waitGroup.Go(func() {
			symbolReports[index] = kCandleIngestionService.ingestSymbol(ingestionDomain, symbol, windowOf)
		})
	}
	waitGroup.Wait()

	return dto.KCandleIngestionReportDto{SymbolReports: symbolReports}
}

// ingestSymbol carries one trading symbol from its window to what was stored. A
// source that will not answer ends this symbol; a single candle that breaks a rule
// only ends itself.
func (kCandleIngestionService *KCandleIngestionService) ingestSymbol(
	ingestionDomain domains.KCandleIngestionDomain,
	symbol string,
	windowOf func(symbol string) (vo.KCandleFetchWindowVo, error),
) dto.KCandleSymbolIngestionReportDto {
	window, windowError := windowOf(symbol)
	if windowError != nil {
		return dto.KCandleSymbolIngestionReportDto{Symbol: symbol, FetchFailureReason: windowError.Error()}
	}

	if window.IsEmpty() {
		return dto.KCandleSymbolIngestionReportDto{Symbol: symbol}
	}

	reportedKCandles, fetchError := kCandleIngestionService.marketDataProxy.FetchKCandles(window)
	if fetchError != nil {
		return dto.KCandleSymbolIngestionReportDto{Symbol: symbol, FetchFailureReason: fetchError.Error()}
	}

	symbolReport := dto.KCandleSymbolIngestionReportDto{
		Symbol:          symbol,
		SkippedKCandles: make([]dto.SkippedKCandleDto, 0),
	}
	for _, reportedKCandle := range ingestionDomain.SelectClosed(reportedKCandles) {
		kCandleDomain, validationError := domains.NewKCandleDomain(
			reportedKCandle.ToWriteDto(), ingestionDomain.CurrentTime())
		if validationError != nil {
			symbolReport.SkippedKCandles = append(symbolReport.SkippedKCandles, dto.SkippedKCandleDto{
				OpenTime: reportedKCandle.OpenTime.UTC(),
				Reason:   validationError.Error(),
			})
			continue
		}

		if _, saveError := kCandleIngestionService.kCandleRepository.Save(kCandleDomain.ToEntity()); saveError != nil {
			symbolReport.SkippedKCandles = append(symbolReport.SkippedKCandles, dto.SkippedKCandleDto{
				OpenTime: reportedKCandle.OpenTime.UTC(),
				Reason:   saveError.Error(),
			})
			continue
		}

		symbolReport.StoredCount++
	}

	return symbolReport
}

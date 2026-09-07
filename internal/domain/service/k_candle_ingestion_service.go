package service

import (
	"context"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleIngestionService keeps the stored K candles current without anyone asking.
// Its two public use cases never call one another: the backfill closes the gap left
// while nothing was running, the periodic round keeps up with the market afterwards.
//
// Both funnel into one path, because they differ only in the stretch of time they
// ask the source for. Whatever the source answers is judged by the ordinary K candle
// rules, one candle at a time.
//
// It reads the watchlist itself at the start of every run rather than being handed
// one. That is what makes a change to the watchlist take effect within a round
// instead of within a restart, and it is also what keeps a run already under way
// working from the list it started with — the read happens once, at the top.
//
// It remembers which markets it has decided are shut for the day. State in a domain
// service is not new here; the live follow registry established the same thing for
// the same reason. What it holds is a conclusion about the world rather than a
// mechanism, and the rule for when the day rolls over stays on the market itself.
type KCandleIngestionService struct {
	kCandleRepository       domaininterface.IKCandleRepository
	tradingSymbolRepository domaininterface.ITradingSymbolRepository
	marketDataProxy         domaininterface.IMarketDataProxy
	clockProxy              domaininterface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	roundCandleCount        int
	backfillLookback        time.Duration

	mutex                      sync.Mutex
	presumedClosedTradingDates map[vo.MarketVo]time.Time
}

func NewKCandleIngestionService(
	kCandleRepository domaininterface.IKCandleRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	marketDataProxy domaininterface.IMarketDataProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	roundCandleCount int,
	backfillLookback time.Duration,
) *KCandleIngestionService {
	return &KCandleIngestionService{
		kCandleRepository:          kCandleRepository,
		tradingSymbolRepository:    tradingSymbolRepository,
		marketDataProxy:            marketDataProxy,
		clockProxy:                 clockProxy,
		marketCatalogDomain:        marketCatalogDomain,
		roundCandleCount:           roundCandleCount,
		backfillLookback:           backfillLookback,
		presumedClosedTradingDates: make(map[vo.MarketVo]time.Time),
	}
}

// RunScheduledRound fetches the newest closed candles for every watched symbol and
// stores them, replacing whatever was held for the same open time. It reports what
// happened rather than failing: one symbol the source would not answer for must not
// take the others down with it.
func (kCandleIngestionService *KCandleIngestionService) RunScheduledRound(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	watchedSymbols, ingestionDomain, prepareError := kCandleIngestionService.prepareRun(executionContext)
	if prepareError != nil {
		return dto.KCandleIngestionReportDto{}, prepareError
	}

	return kCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error) {
			return ingestionDomain.ScheduledWindow(watchedSymbol.Symbol, market), nil
		}), nil
}

// RunBackfill closes each watched symbol's gap, reaching no further back than the
// lookback allows. A symbol already up to date is left alone and its source is never
// called.
func (kCandleIngestionService *KCandleIngestionService) RunBackfill(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	watchedSymbols, ingestionDomain, prepareError := kCandleIngestionService.prepareRun(executionContext)
	if prepareError != nil {
		return dto.KCandleIngestionReportDto{}, prepareError
	}

	return kCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error) {
			latestStored, findError := kCandleIngestionService.kCandleRepository.FindLatest(
				executionContext, watchedSymbol.Symbol, 1)
			if findError != nil {
				return vo.KCandleFetchWindowVo{}, findError
			}

			if len(latestStored) == 0 {
				return ingestionDomain.BackfillWindow(watchedSymbol.Symbol, market, time.Time{}), nil
			}

			return ingestionDomain.BackfillWindow(
				watchedSymbol.Symbol, market, latestStored[0].OpenTime), nil
		}), nil
}

// prepareRun reads the clock and the watchlist once each, so that every window in one
// run and every candle judged during it share one list and one idea of "now".
//
// The rules are settled before the watchlist is read. A run that cannot work whatever
// it is pointed at should not reach storage first, and reading a list it is about to
// throw away would say the opposite about what went wrong.
func (kCandleIngestionService *KCandleIngestionService) prepareRun(
	executionContext context.Context,
) ([]entities.TradingSymbol, domains.KCandleIngestionDomain, error) {
	ingestionDomain, buildError := domains.NewKCandleIngestionDomain(
		kCandleIngestionService.clockProxy.Now(),
		kCandleIngestionService.roundCandleCount,
		kCandleIngestionService.backfillLookback,
	)
	if buildError != nil {
		return nil, domains.KCandleIngestionDomain{}, buildError
	}

	watchedSymbols, findError := kCandleIngestionService.tradingSymbolRepository.FindWatched(
		executionContext)
	if findError != nil {
		return nil, domains.KCandleIngestionDomain{}, findError
	}

	return watchedSymbols, ingestionDomain, nil
}

// ingestSymbols runs every watched symbol at once. A plain wait group is deliberate:
// an error group would cancel the remaining symbols the moment one failed, which is
// the opposite of what independence per symbol means here. Each goroutine owns one
// slot of the result, so nothing is shared and nothing needs locking.
func (kCandleIngestionService *KCandleIngestionService) ingestSymbols(
	executionContext context.Context,
	watchedSymbols []entities.TradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	windowOf func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error),
) dto.KCandleIngestionReportDto {
	symbolReports := make([]dto.KCandleSymbolIngestionReportDto, len(watchedSymbols))

	var waitGroup sync.WaitGroup
	for index, watchedSymbol := range watchedSymbols {
		waitGroup.Go(func() {
			symbolReports[index] = kCandleIngestionService.ingestSymbol(
				executionContext, watchedSymbol, ingestionDomain, windowOf)
		})
	}
	waitGroup.Wait()

	kCandleIngestionService.presumeClosedMarkets(symbolReports, ingestionDomain.CurrentTime())

	return dto.KCandleIngestionReportDto{SymbolReports: symbolReports}
}

// ingestSymbol carries one trading symbol from its window to what was stored. A
// source that will not answer ends this symbol; a single candle that breaks a rule
// only ends itself.
//
// The window is narrowed to what its market could actually hold before anything is
// asked for, and that one step is the whole of "a closed market is not a failure": a
// night, a weekend and a day already decided shut all shrink the window to nothing,
// and a window covering nothing has always meant there is nothing to do. It is also
// why the round just after the close still collects the day's last candle — the
// window still overlaps the session even though the market no longer does.
func (kCandleIngestionService *KCandleIngestionService) ingestSymbol(
	executionContext context.Context,
	watchedSymbol entities.TradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	windowOf func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error),
) dto.KCandleSymbolIngestionReportDto {
	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(watchedSymbol.Market)
	symbolReport := dto.KCandleSymbolIngestionReportDto{
		Symbol: watchedSymbol.Symbol,
		Market: string(marketDomain.Value()),
	}

	if kCandleIngestionService.isPresumedClosed(marketDomain, ingestionDomain.CurrentTime()) {
		return symbolReport
	}

	window, windowError := windowOf(watchedSymbol, marketDomain.Value())
	if windowError != nil {
		symbolReport.FetchFailureReason = windowError.Error()

		return symbolReport
	}

	tradableWindow := marketDomain.ClampToTradingSession(window)
	if tradableWindow.IsEmpty() {
		return symbolReport
	}

	reportedKCandles, fetchError := kCandleIngestionService.marketDataProxy.FetchKCandles(
		executionContext, tradableWindow)
	if fetchError != nil {
		symbolReport.FetchFailureReason = fetchError.Error()

		return symbolReport
	}

	symbolReport.WasAsked = true
	symbolReport.SkippedKCandles = make([]dto.SkippedKCandleDto, 0)
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

		if _, saveError := kCandleIngestionService.kCandleRepository.Save(
			executionContext, kCandleDomain.ToEntity()); saveError != nil {
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

// presumeClosedMarkets decides which markets were shut today, from what the round
// just saw: the source answered for every symbol of that market, and had not one
// candle for any of them.
//
// Holidays are read off the market's own answers rather than kept in a calendar
// somebody has to maintain, because an unmaintained holiday list is wrong on exactly
// the days it matters. Requiring every symbol of a market to come back empty is what
// stops one quiet stock from shutting the whole market: a real holiday empties all of
// them at once.
//
// A source that could not be reached is never read as a holiday. Answered-with-
// nothing versus did-not-answer is the only reliable distinction available here, and
// it is what leaves a broken source reported as broken.
func (kCandleIngestionService *KCandleIngestionService) presumeClosedMarkets(
	symbolReports []dto.KCandleSymbolIngestionReportDto, currentTime time.Time,
) {
	askedMarkets := make(map[vo.MarketVo]bool)
	marketsThatProduced := make(map[vo.MarketVo]bool)
	for _, symbolReport := range symbolReports {
		if !symbolReport.WasAsked {
			continue
		}

		market := vo.MarketVo(symbolReport.Market)
		askedMarkets[market] = true
		if symbolReport.StoredCount > 0 || len(symbolReport.SkippedKCandles) > 0 {
			marketsThatProduced[market] = true
		}
	}

	if len(askedMarkets) == 0 {
		return
	}

	kCandleIngestionService.mutex.Lock()
	defer kCandleIngestionService.mutex.Unlock()

	for market := range askedMarkets {
		if marketsThatProduced[market] {
			continue
		}

		marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(string(market))
		// A market with no hours has no days off either. Reading a quiet stretch as a
		// holiday would stop it being fetched until a tomorrow it does not have.
		if marketDomain.NeverCloses() {
			continue
		}

		kCandleIngestionService.presumedClosedTradingDates[market] =
			marketDomain.TradingDateOf(currentTime)
	}
}

// isPresumedClosed reports a market already decided to be shut for the day it is
// currently in. Asking the market which day that is, rather than reading a calendar
// here, is what makes the decision last exactly until the market's own tomorrow.
func (kCandleIngestionService *KCandleIngestionService) isPresumedClosed(
	marketDomain domains.MarketDomain, currentTime time.Time,
) bool {
	kCandleIngestionService.mutex.Lock()
	defer kCandleIngestionService.mutex.Unlock()

	presumedClosedOn, wasPresumedClosed :=
		kCandleIngestionService.presumedClosedTradingDates[marketDomain.Value()]
	if !wasPresumedClosed {
		return false
	}

	return presumedClosedOn.Equal(marketDomain.TradingDateOf(currentTime))
}

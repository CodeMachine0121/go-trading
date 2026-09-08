package service

import (
	"context"
	"fmt"
	"strings"
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
// Which markets it has decided are shut for the day is remembered for it, by the
// one thing that holds that memory and nothing else. So this service keeps no shared
// state and locks nothing: what looks shut is read from what a round saw, when the
// decision expires is the market's own calendar, and neither of those is here.
type KCandleIngestionService struct {
	kCandleRepository       domaininterface.IKCandleRepository
	tradingSymbolRepository domaininterface.ITradingSymbolRepository
	marketDataProxy         domaininterface.IMarketDataProxy
	clockProxy              domaininterface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	roundCandleCount        int
	backfillLookback        time.Duration
	marketClosureLedger     *marketClosureLedger
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
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		marketDataProxy:         marketDataProxy,
		clockProxy:              clockProxy,
		marketCatalogDomain:     marketCatalogDomain,
		roundCandleCount:        roundCandleCount,
		backfillLookback:        backfillLookback,
		marketClosureLedger:     newMarketClosureLedger(),
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

	report := kCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error) {
			return ingestionDomain.ScheduledWindow(watchedSymbol.Symbol, market), nil
		})
	kCandleIngestionService.presumeClosedMarkets(
		report.SymbolReports, ingestionDomain.CurrentTime(), ingestionDomain.RoundCoverage())

	return report, nil
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

	report := kCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		kCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain))
	kCandleIngestionService.presumeClosedMarkets(
		report.SymbolReports, ingestionDomain.CurrentTime(), ingestionDomain.RoundCoverage())

	return report, nil
}

// RunBackfillFor closes one trading symbol's gap on demand, reaching no further back
// than the lookback allows.
//
// It exists because the two automatic runs are both driven by time — one by the
// clock at startup, one by a tick as long as a K candle — and neither can answer "I want this
// symbol's day now". A market that closes makes that gap visible: after the bell,
// the scheduled round has nothing left to collect, so a symbol nobody had before the
// close would have no candles at all until the next start-up.
//
// It reaches the symbol by name rather than through the watchlist, because a chart
// can be opened for a symbol nobody is watching, and catching that one up is exactly
// what somebody looking at it is asking for.
//
// It deliberately never presumes a market shut. That conclusion is only earned by
// asking every one of a market's watched symbols and hearing nothing from any of
// them; one symbol's silence is one symbol's, and treating it as the market's would
// stop the whole market being fetched for the rest of the day.
func (kCandleIngestionService *KCandleIngestionService) RunBackfillFor(
	executionContext context.Context, symbol string,
) (dto.KCandleIngestionReportDto, error) {
	tradingSymbolDomain, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return dto.KCandleIngestionReportDto{}, fmt.Errorf("%w: %w",
			domains.ErrTradingSymbolNamed, symbolError)
	}

	ingestionDomain, buildError := kCandleIngestionService.buildIngestionDomain()
	if buildError != nil {
		return dto.KCandleIngestionReportDto{}, buildError
	}

	registeredSymbol, isRegistered, findError := kCandleIngestionService.tradingSymbolRepository.
		FindBySymbol(executionContext, tradingSymbolDomain.Value())
	if findError != nil {
		return dto.KCandleIngestionReportDto{}, findError
	}

	// Without a registration there is no market, and without a market there is no
	// source to ask. Guessing one from the shape of the name is the rule this system
	// deliberately does not have.
	if !isRegistered {
		return dto.KCandleIngestionReportDto{}, fmt.Errorf("%w: %s",
			domains.ErrTradingSymbolNotRegistered, tradingSymbolDomain.Value())
	}

	// Somebody asking by hand is somebody saying they want the source asked. Obeying a
	// presumed holiday here would answer them with a report saying nothing was
	// collected — indistinguishable from a market that genuinely had nothing — and
	// leave them no way to correct a decision that may have been wrong in the first
	// place. So the decision is dropped rather than obeyed; if the market really is
	// shut, the window narrows to nothing and the next round decides it shut again.
	kCandleIngestionService.marketClosureLedger.reconsider(
		kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market).Value())

	return kCandleIngestionService.ingestSymbols(
		executionContext,
		[]entities.TradingSymbol{registeredSymbol},
		ingestionDomain,
		kCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain),
	), nil
}

// backfillWindowOf is how far back one symbol has to be asked about: from wherever
// its own history left off, and no further back than the lookback allows.
//
// It is shared by the run over the whole watchlist and the run over a single symbol
// because "how much of this symbol is missing" is the same question either way —
// only the list of symbols differs.
func (kCandleIngestionService *KCandleIngestionService) backfillWindowOf(
	executionContext context.Context, ingestionDomain domains.KCandleIngestionDomain,
) func(entities.TradingSymbol, vo.MarketVo) (vo.KCandleFetchWindowVo, error) {
	return func(
		watchedSymbol entities.TradingSymbol, market vo.MarketVo,
	) (vo.KCandleFetchWindowVo, error) {
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
	}
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
	ingestionDomain, buildError := kCandleIngestionService.buildIngestionDomain()
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

// buildIngestionDomain settles the rules against one reading of the clock. Every run
// starts here, so a run that cannot work whatever it is pointed at says so before it
// has read a watchlist or touched a source.
func (kCandleIngestionService *KCandleIngestionService) buildIngestionDomain() (
	domains.KCandleIngestionDomain, error,
) {
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

	if kCandleIngestionService.marketClosureLedger.isPresumedClosed(
		marketDomain.Value(), marketDomain.TradingDateOf(ingestionDomain.CurrentTime())) {
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
	symbolReports []dto.KCandleSymbolIngestionReportDto,
	currentTime time.Time,
	roundCoverage time.Duration,
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

		// An empty answer only means something once the market has had at least as
		// long to speak as the round asked about. Two minutes after the opening bell a
		// round asks about one candle, and a source that publishes it a moment late
		// empties every symbol at once — which is exactly the shape of a holiday. Given
		// that latching one costs the market the rest of its day, it has to wait until
		// silence is actually evidence.
		if marketDomain.SessionElapsedAt(currentTime) < roundCoverage {
			continue
		}

		kCandleIngestionService.marketClosureLedger.presumeClosed(
			market, marketDomain.TradingDateOf(currentTime))
	}
}

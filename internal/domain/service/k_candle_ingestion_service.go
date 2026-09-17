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

// kCandleWriteRule is what a run does about a minute it already holds a candle for.
//
// The two are genuinely different intentions rather than one with a setting. An
// automatic round collects the candle of the minute that just closed and means to
// replace it next time round, once the source has settled its figures; filling a gap
// means never touching what is already there, because the caller asked for the
// missing ones and nothing else.
type kCandleWriteRule int

const (
	// replaceStoredKCandles is what every automatic path does, and the manual
	// catch-up with it: the newer answer from the source wins.
	replaceStoredKCandles kCandleWriteRule = iota
	// keepStoredKCandles fills in only the minutes nothing is held for.
	keepStoredKCandles
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
	marketClosureLedger     *kCandleIngestionMarketClosureLedger
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
		marketClosureLedger:     newKCandleIngestionMarketClosureLedger(),
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
		}, replaceStoredKCandles)
	kCandleIngestionService.presumeClosedMarkets(
		report.SymbolReports, ingestionDomain.CurrentTime(), ingestionDomain.RoundCoverage(),
		replaceStoredKCandles)

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
		kCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain),
		replaceStoredKCandles)
	kCandleIngestionService.presumeClosedMarkets(
		report.SymbolReports, ingestionDomain.CurrentTime(), ingestionDomain.RoundCoverage(),
		replaceStoredKCandles)

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
	registeredSymbol, ingestionDomain, reachError := kCandleIngestionService.reachSymbolOnDemand(
		executionContext, symbol)
	if reachError != nil {
		return dto.KCandleIngestionReportDto{}, reachError
	}

	return kCandleIngestionService.ingestSymbols(
		executionContext,
		[]entities.TradingSymbol{registeredSymbol},
		ingestionDomain,
		kCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain),
		replaceStoredKCandles,
	), nil
}

// SyncHistoryFor fills in the minutes missing from a stretch of one trading symbol's
// history that somebody named.
//
// It is not a backfill with an argument. A backfill starts wherever the stored data
// left off, so a minute missing from the *middle* of a stretch is one it never comes
// back for — holding day one and day thirty, it begins after day thirty. This asks
// about the whole stretch, every time, which is the only way that hole gets filled.
//
// **What it asks about and what it writes are two different things.** It asks about
// everything and writes only what is absent: a candle already held is left exactly as
// it is. So a stretch that was already complete reports nothing stored, which is the
// truth about what this run changed.
//
// It shares every gate with the on-demand catch-up — the name, the registration, the
// dropped closure decision — because those are questions about the symbol rather than
// about the stretch, and a symbol is a symbol whichever way it is being fetched.
//
// The ceiling arrives as an argument rather than being held by this service, for the
// same reason every other ceiling here does not: how far this system is willing to
// reach in one request is an operator's decision, and a service that remembered it
// would be a second place for it to be wrong.
func (kCandleIngestionService *KCandleIngestionService) SyncHistoryFor(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, ceilingDays int,
) (dto.KCandleIngestionReportDto, error) {
	// Judged before anything is read. A request that cannot work should not reach
	// storage first, and the caller with a lookback of nothing is told about the
	// lookback rather than about a symbol they spelled perfectly well.
	lookback, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		syncDto.LookbackDays, ceilingDays)
	if lookbackError != nil {
		return dto.KCandleIngestionReportDto{}, lookbackError
	}

	registeredSymbol, ingestionDomain, reachError := kCandleIngestionService.reachSymbolOnDemand(
		executionContext, syncDto.Symbol)
	if reachError != nil {
		return dto.KCandleIngestionReportDto{}, reachError
	}

	return kCandleIngestionService.ingestSymbols(
		executionContext,
		[]entities.TradingSymbol{registeredSymbol},
		ingestionDomain,
		func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error) {
			return ingestionDomain.HistoryWindow(
				watchedSymbol.Symbol, market, lookback.Duration()), nil
		},
		keepStoredKCandles,
	), nil
}

// reachSymbolOnDemand is everything the two hand-driven fetches do before they differ:
// judge the name, settle the rules, find the registration, and drop any standing
// decision that the symbol's market is shut for the day.
//
// It is shared because both of them need every step, and because the steps are about
// the symbol rather than about the stretch — the two differ only in which window they
// end up asking for.
//
// **Dropping the closure decision is the deliberate part.** Somebody asking by hand is
// somebody saying they want the source asked. Obeying a presumed holiday would answer
// them with a report saying nothing was collected — indistinguishable from a market
// that genuinely had nothing — and leave them no way to correct a decision that may
// have been wrong in the first place.
func (kCandleIngestionService *KCandleIngestionService) reachSymbolOnDemand(
	executionContext context.Context, symbol string,
) (entities.TradingSymbol, domains.KCandleIngestionDomain, error) {
	tradingSymbolDomain, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return entities.TradingSymbol{}, domains.KCandleIngestionDomain{}, fmt.Errorf("%w: %w",
			domains.ErrTradingSymbolNamed, symbolError)
	}

	ingestionDomain, buildError := kCandleIngestionService.buildIngestionDomain()
	if buildError != nil {
		return entities.TradingSymbol{}, domains.KCandleIngestionDomain{}, buildError
	}

	registeredSymbol, isRegistered, findError := kCandleIngestionService.tradingSymbolRepository.
		FindBySymbol(executionContext, tradingSymbolDomain.Value())
	if findError != nil {
		return entities.TradingSymbol{}, domains.KCandleIngestionDomain{}, findError
	}

	// Without a registration there is no market, and without a market there is no
	// source to ask. Guessing one from the shape of the name is the rule this system
	// deliberately does not have.
	if !isRegistered {
		return entities.TradingSymbol{}, domains.KCandleIngestionDomain{}, fmt.Errorf("%w: %s",
			domains.ErrTradingSymbolNotRegistered, tradingSymbolDomain.Value())
	}

	kCandleIngestionService.marketClosureLedger.reconsider(
		kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market).Value())

	return registeredSymbol, ingestionDomain, nil
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
	writeRule kCandleWriteRule,
) dto.KCandleIngestionReportDto {
	symbolReports := make([]dto.KCandleSymbolIngestionReportDto, len(watchedSymbols))

	var waitGroup sync.WaitGroup
	for index, watchedSymbol := range watchedSymbols {
		waitGroup.Go(func() {
			symbolReports[index] = kCandleIngestionService.ingestSymbol(
				executionContext, watchedSymbol, ingestionDomain, windowOf, writeRule)
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
	writeRule kCandleWriteRule,
) dto.KCandleSymbolIngestionReportDto {
	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(watchedSymbol.Market)
	// The skipped list is built here rather than where the first candle is judged,
	// so that every way out of this function answers with a list. Left to appear only
	// on the path that reaches the source, it would be absent on exactly the paths a
	// reader inspects it on — a shut market, a source that would not answer — and a
	// caller counting it would break there and nowhere else.
	symbolReport := dto.KCandleSymbolIngestionReportDto{
		Symbol:          watchedSymbol.Symbol,
		Market:          string(marketDomain.Value()),
		SkippedKCandles: make([]dto.SkippedKCandleDto, 0),
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

		stored, saveError := kCandleIngestionService.store(
			executionContext, kCandleDomain.ToEntity(), writeRule)
		if saveError != nil {
			symbolReport.SkippedKCandles = append(symbolReport.SkippedKCandles, dto.SkippedKCandleDto{
				OpenTime: reportedKCandle.OpenTime.UTC(),
				Reason:   saveError.Error(),
			})
			continue
		}

		// A minute already held under the keep rule is neither stored nor skipped:
		// nothing went wrong and nothing was written. It is left out of both counts
		// rather than given a third, because the question the report answers is what
		// this run changed.
		if !stored {
			continue
		}

		symbolReport.StoredCount++
	}

	return symbolReport
}

// store writes one candle the way this run was told to, and says whether anything was
// actually written.
//
// Replacing always writes, so it always reports having done so; keeping reports what
// the store decided, because whether that minute was already held is the store's to
// know and nobody else's.
func (kCandleIngestionService *KCandleIngestionService) store(
	executionContext context.Context, kCandle entities.KCandle, writeRule kCandleWriteRule,
) (bool, error) {
	if writeRule == keepStoredKCandles {
		return kCandleIngestionService.kCandleRepository.SaveIfAbsent(executionContext, kCandle)
	}

	if _, saveError := kCandleIngestionService.kCandleRepository.Save(
		executionContext, kCandle); saveError != nil {
		return false, saveError
	}

	return true, nil
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
//
// **It can only read a run that overwrote what it collected**, which is why the rule
// that run used is an argument rather than an assumption. What it reads a holiday off
// is a symbol that was asked and produced nothing — and under the keep rule a symbol
// whose candles are all already held produces nothing either, while saying nothing at
// all about the market. Letting that through would latch a perfectly open market shut
// for the rest of the day, on the evidence that somebody asked for history they
// already had.
func (kCandleIngestionService *KCandleIngestionService) presumeClosedMarkets(
	symbolReports []dto.KCandleSymbolIngestionReportDto,
	currentTime time.Time,
	roundCoverage time.Duration,
	writeRule kCandleWriteRule,
) {
	if writeRule != replaceStoredKCandles {
		return
	}

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

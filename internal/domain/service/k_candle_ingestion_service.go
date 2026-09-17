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

// maxNamedSkippedKCandles is how many skipped candles one symbol's report names
// before it stops naming them and only counts them.
//
// It is generous on purpose. Anything short of a broken source produces a handful,
// and a handful is what somebody reads a report for; the limit is only there so that
// a source answering with rubbish for four years cannot turn the report into
// something nobody can open.
const maxNamedSkippedKCandles = 200

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
	kCandleRepository               domaininterface.IKCandleRepository
	kCandleHistorySyncRunRepository domaininterface.IKCandleHistorySyncRunRepository
	tradingSymbolRepository         domaininterface.ITradingSymbolRepository
	marketDataProxy                 domaininterface.IMarketDataProxy
	clockProxy                      domaininterface.IClockProxy
	marketCatalogDomain             domains.MarketCatalogDomain
	roundCandleCount                int
	backfillLookback                time.Duration
	marketClosureLedger             *kCandleIngestionMarketClosureLedger
}

func NewKCandleIngestionService(
	kCandleRepository domaininterface.IKCandleRepository,
	kCandleHistorySyncRunRepository domaininterface.IKCandleHistorySyncRunRepository,
	tradingSymbolRepository domaininterface.ITradingSymbolRepository,
	marketDataProxy domaininterface.IMarketDataProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	roundCandleCount int,
	backfillLookback time.Duration,
) *KCandleIngestionService {
	return &KCandleIngestionService{
		kCandleRepository:               kCandleRepository,
		kCandleHistorySyncRunRepository: kCandleHistorySyncRunRepository,
		tradingSymbolRepository:         tradingSymbolRepository,
		marketDataProxy:                 marketDataProxy,
		clockProxy:                      clockProxy,
		marketCatalogDomain:             marketCatalogDomain,
		roundCandleCount:                roundCandleCount,
		backfillLookback:                backfillLookback,
		marketClosureLedger:             newKCandleIngestionMarketClosureLedger(),
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
	), nil
}

// StartHistorySyncFor accepts a request to fill in the minutes missing from a stretch
// of one trading symbol's history, and answers with where to watch it happen.
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
//
// **It answers before the fetching starts, and that is not an optimisation.** Four
// years of one-minute candles is thousands of requests to a source that is paced to
// what it allows — tens of minutes of fetching, and longer on a market answering one
// day at a time. There is no connection worth holding open that long: a proxy, a load
// balancer or a laptop lid closing would end the request and, with a synchronous
// fetch, the work with it. So the run is written down first, the work is driven by
// something that outlives the request, and the caller is handed the run to come back
// and look at.
//
// Everything that can refuse the request still refuses it here, before anything is
// recorded — a lookback that is not a stretch, a name that is not one, a symbol
// nobody registered. A run recorded and then immediately failed would be a worse
// answer to those than a refusal.
func (kCandleIngestionService *KCandleIngestionService) StartHistorySyncFor(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, ceilingDays int,
) (dto.KCandleHistorySyncRunDto, error) {
	// Judged before anything is read. A request that cannot work should not reach
	// storage first, and the caller with a lookback of nothing is told about the
	// lookback rather than about a symbol they spelled perfectly well.
	lookback, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		syncDto.LookbackDays, ceilingDays)
	if lookbackError != nil {
		return dto.KCandleHistorySyncRunDto{}, lookbackError
	}

	registeredSymbol, ingestionDomain, reachError := kCandleIngestionService.reachSymbolOnDemand(
		executionContext, syncDto.Symbol)
	if reachError != nil {
		return dto.KCandleHistorySyncRunDto{}, reachError
	}

	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market)
	chunks := ingestionDomain.HistoryChunks(
		registeredSymbol.Symbol, marketDomain.Value(), lookback.Duration())

	syncRun, saveError := kCandleIngestionService.kCandleHistorySyncRunRepository.Save(
		executionContext, entities.KCandleHistorySyncRun{
			Symbol:       registeredSymbol.Symbol,
			LookbackDays: syncDto.LookbackDays,
			Status:       string(vo.KCandleHistorySyncRunning),
			TotalChunks:  len(chunks),
			StartedAt:    ingestionDomain.CurrentTime(),
		})
	if saveError != nil {
		// Nothing is started. A run nobody can find is work nobody can ask about and
		// a restart cannot sweep up, which is worse than not having begun.
		return dto.KCandleHistorySyncRunDto{}, saveError
	}

	go (&kCandleHistorySyncRunner{
		kCandleIngestionService: kCandleIngestionService,
		syncRun:                 syncRun,
		registeredSymbol:        registeredSymbol,
		ingestionDomain:         ingestionDomain,
		chunks:                  chunks,
	}).run()

	return syncRun.ToDto(), nil
}

// GetHistorySyncRun answers with where one history sync has got to.
func (kCandleIngestionService *KCandleIngestionService) GetHistorySyncRun(
	executionContext context.Context, id uint,
) (dto.KCandleHistorySyncRunDto, error) {
	syncRun, found, findError := kCandleIngestionService.kCandleHistorySyncRunRepository.FindOne(
		executionContext, id)
	if findError != nil {
		return dto.KCandleHistorySyncRunDto{}, findError
	}
	if !found {
		return dto.KCandleHistorySyncRunDto{}, ErrKCandleHistorySyncRunNotFound
	}

	return syncRun.ToDto(), nil
}

// FailInterruptedHistorySyncs clears out the runs the last shutdown cut off, and says
// how many there were.
//
// A run being fetched lives in this process and nowhere else, so every one still
// recorded as running is stale the moment this one starts. Left alone each is a
// progress figure that never moves again.
func (kCandleIngestionService *KCandleIngestionService) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return kCandleIngestionService.kCandleHistorySyncRunRepository.FailAllRunning(
		executionContext, kCandleHistorySyncInterrupted,
		kCandleIngestionService.clockProxy.Now())
}

// syncSymbolHistory walks one symbol's stretch a chunk at a time, asking the source
// for a chunk and storing it before moving to the next.
//
// **The chunking is not an optimisation, it is what makes a long stretch possible at
// all.** Asked as one window, four years is around two million candles: every one of
// them is held in memory until the last page arrives, and the whole run is lost to a
// single dropped connection near the end. A chunk at a time keeps the memory flat and
// leaves every chunk already stored behind it — a run that gives up halfway has still
// done half the work, and running it again picks up from there because what is
// already held is never written twice.
//
// **A chunk already complete is not asked about.** Over four years almost every chunk
// is, and the counting read that establishes it is a fraction of the cost of the round
// trip it avoids. This is the one place the history sync looks at what is stored — and
// it looks to decide whether to ask, never to decide where to start. Starting from what
// is stored is what leaves a hole in the middle unreachable, which is the whole reason
// this use case exists.
//
// It does not go through ingestSymbols, and that is deliberate on two counts: there is
// only ever one symbol here, and the closure ledger reached from there must never see
// this run. A stretch already held answers with nothing stored while saying nothing at
// all about the market, and letting that through would latch an open market shut for
// the rest of the day.
func (kCandleIngestionService *KCandleIngestionService) syncSymbolHistory(
	executionContext context.Context,
	registeredSymbol entities.TradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	chunks []vo.KCandleFetchWindowVo,
	recordProgress func(completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto),
) error {
	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market)
	symbolReport := dto.KCandleSymbolIngestionReportDto{
		Symbol:          registeredSymbol.Symbol,
		Market:          string(marketDomain.Value()),
		SkippedKCandles: make([]dto.SkippedKCandleDto, 0),
	}

	// Everything this walk has to say travels back through recordProgress rather than
	// through the return, so that **whatever it managed before it stopped is already
	// written down**. A walk that gives up at chunk five of fifteen hundred has still
	// stored five chunks' worth, and a caller handed an error and nothing else would
	// have to report that as zero.
	for chunkIndex, chunk := range chunks {
		recordProgress(chunkIndex, symbolReport)

		tradableChunk := marketDomain.ClampToTradingSession(chunk)
		if tradableChunk.IsEmpty() {
			continue
		}

		alreadyHeld, countError := kCandleIngestionService.kCandleRepository.CountInRange(
			executionContext, registeredSymbol.Symbol,
			tradableChunk.StartTime, tradableChunk.EndTime)
		if countError != nil {
			recordProgress(chunkIndex, symbolReport)

			return countError
		}

		if alreadyHeld >= marketDomain.TradingKCandleCountBetween(
			tradableChunk.StartTime, tradableChunk.EndTime) {
			continue
		}

		reportedKCandles, fetchError := kCandleIngestionService.marketDataProxy.FetchKCandles(
			executionContext, tradableChunk)
		if fetchError != nil {
			// The rest of the stretch is abandoned rather than attempted. A source
			// that just refused one chunk will refuse the next two thousand the same
			// way, and hammering it is how a rate limit becomes a ban. What was
			// stored before this point stays stored — it is correct, and running
			// again resumes from it.
			symbolReport.FetchFailureReason = fetchError.Error()
			recordProgress(chunkIndex, symbolReport)

			return nil
		}

		symbolReport.WasAsked = true

		judgedKCandles, skippedKCandles := kCandleIngestionService.judge(
			reportedKCandles, ingestionDomain)
		kCandleIngestionService.noteSkipped(&symbolReport, skippedKCandles...)

		storedCount, saveError := kCandleIngestionService.kCandleRepository.SaveAllIfAbsent(
			executionContext, judgedKCandles)
		if saveError != nil {
			// Storage breaking is this system's own fault, not this chunk's, so it
			// ends the run rather than being written down as something the candles
			// did wrong. What the earlier chunks stored is reported on the way out:
			// it is in the database, and a run claiming otherwise would send somebody
			// back to fetch it all again.
			recordProgress(chunkIndex, symbolReport)

			return saveError
		}

		symbolReport.StoredCount += storedCount
	}

	recordProgress(len(chunks), symbolReport)

	return nil
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

	judgedKCandles, skippedKCandles := kCandleIngestionService.judge(
		reportedKCandles, ingestionDomain)
	kCandleIngestionService.noteSkipped(&symbolReport, skippedKCandles...)

	for _, judgedKCandle := range judgedKCandles {
		if _, saveError := kCandleIngestionService.kCandleRepository.Save(
			executionContext, judgedKCandle); saveError != nil {
			kCandleIngestionService.noteSkipped(&symbolReport, dto.SkippedKCandleDto{
				OpenTime: judgedKCandle.OpenTime.UTC(),
				Reason:   saveError.Error(),
			})
			continue
		}

		symbolReport.StoredCount++
	}

	return symbolReport
}

// noteSkipped writes candles that did not make it into the report, counting all of
// them and naming them up to the limit.
//
// Every path writes a skip through here so that the count and the list mean the same
// thing wherever they are read. Naming them all was fine while a run covered minutes;
// a run covering years can turn up more than anybody will read, and the count answers
// "how bad is it" without the list having to.
func (kCandleIngestionService *KCandleIngestionService) noteSkipped(
	symbolReport *dto.KCandleSymbolIngestionReportDto, skippedKCandles ...dto.SkippedKCandleDto,
) {
	for _, skippedKCandle := range skippedKCandles {
		symbolReport.SkippedCount++

		if len(symbolReport.SkippedKCandles) >= maxNamedSkippedKCandles {
			symbolReport.SkippedKCandlesTruncated = true

			continue
		}

		symbolReport.SkippedKCandles = append(symbolReport.SkippedKCandles, skippedKCandle)
	}
}

// judge puts everything the source answered with through the ordinary K candle rules,
// handing back the ones that may be stored and naming the ones that may not.
//
// It is shared by the automatic paths and the history sync, because what makes a
// candle acceptable has nothing to do with why it was fetched. What they do with the
// result differs — one replaces a candle at a time, the other writes a day's worth of
// absent ones in a single statement — and that difference is theirs, not this one's.
func (kCandleIngestionService *KCandleIngestionService) judge(
	reportedKCandles []vo.MarketKCandleVo, ingestionDomain domains.KCandleIngestionDomain,
) ([]entities.KCandle, []dto.SkippedKCandleDto) {
	closedKCandles := ingestionDomain.SelectClosed(reportedKCandles)

	judgedKCandles := make([]entities.KCandle, 0, len(closedKCandles))
	skippedKCandles := make([]dto.SkippedKCandleDto, 0)

	for _, reportedKCandle := range closedKCandles {
		kCandleDomain, validationError := domains.NewKCandleDomain(
			reportedKCandle.ToWriteDto(), ingestionDomain.CurrentTime())
		if validationError != nil {
			skippedKCandles = append(skippedKCandles, dto.SkippedKCandleDto{
				OpenTime: reportedKCandle.OpenTime.UTC(),
				Reason:   validationError.Error(),
			})
			continue
		}

		judgedKCandles = append(judgedKCandles, kCandleDomain.ToEntity())
	}

	return judgedKCandles, skippedKCandles
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
// **Only the runs that overwrite what they collect may read this**, and the history
// sync is kept out of its reach by not going through ingestSymbols at all. What it
// reads a holiday off is a symbol that was asked and produced nothing — and a history
// sync of a stretch already held produces nothing either, while saying nothing at all
// about the market. Letting that through would latch a perfectly open market shut for
// the rest of the day, on the evidence that somebody asked for history they already
// had.
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

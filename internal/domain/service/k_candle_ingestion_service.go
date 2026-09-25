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

// KCandleIngestionService keeps stored K candles current via a startup backfill and a periodic round, both funnelled through one path.
// The watchlist is read once at the start of each run, so changes apply on the next round and a running round keeps its list.
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

// RunScheduledRound fetches and upserts the newest closed candles for every watched symbol, reporting per-symbol failures rather than failing the round.
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

// RunBackfill closes each watched symbol's gap within the lookback, skipping symbols already up to date.
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

// RunBackfillFor closes one symbol's gap on demand, by name rather than via the watchlist, so a chart opened after the close still gets the day's candles.
// It never presumes a market shut, since one symbol's silence is not evidence about the whole market.
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

// StartHistorySyncFor records a run and fetches the whole stretch in the background, filling holes a backfill (which starts after the latest stored candle) can never reach.
// It writes only absent candles, validates everything before recording the run, and returns immediately because a multi-year fetch outlives any request.
func (kCandleIngestionService *KCandleIngestionService) StartHistorySyncFor(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, ceilingDays int,
) (dto.KCandleHistorySyncRunDto, error) {
	// Validated before any read, so a bad lookback is reported as such.
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
		// Without a findable run nothing can observe or sweep the work, so nothing is started.
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

// FailInterruptedHistorySyncs marks every run still recorded as running as failed, since runs live only in the process that started them.
func (kCandleIngestionService *KCandleIngestionService) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return kCandleIngestionService.kCandleHistorySyncRunRepository.FailAllRunning(
		executionContext, kCandleHistorySyncInterrupted,
		kCandleIngestionService.clockProxy.Now())
}

// syncSymbolHistory walks the stretch chunk by chunk so memory stays flat and a failed run keeps the chunks already stored, skipping chunks already complete.
// It bypasses ingestSymbols so the closure ledger never sees it: an already-held stretch stores nothing and would otherwise latch an open market shut.
func (kCandleIngestionService *KCandleIngestionService) syncSymbolHistory(
	executionContext context.Context,
	registeredSymbol entities.TradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	chunks []vo.KCandleFetchWindowVo,
	recordProgress func(completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto),
) error {
	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market)
	symbolReport := domains.NewKCandleSymbolIngestionReportDomain(
		registeredSymbol.Symbol, marketDomain.Value())

	// Progress is reported through recordProgress, not the return, so partial work is recorded even when the walk stops early.
	for chunkIndex, chunk := range chunks {
		recordProgress(chunkIndex, symbolReport.ToDto())

		tradableChunk := marketDomain.ClampToTradingSession(chunk)
		if tradableChunk.IsEmpty() {
			continue
		}

		alreadyHeld, countError := kCandleIngestionService.kCandleRepository.CountInRange(
			executionContext, registeredSymbol.Symbol,
			tradableChunk.StartTime, tradableChunk.EndTime)
		if countError != nil {
			recordProgress(chunkIndex, symbolReport.ToDto())

			return countError
		}

		if alreadyHeld >= marketDomain.TradingKCandleCountBetween(
			tradableChunk.StartTime, tradableChunk.EndTime) {
			continue
		}

		reportedKCandles, fetchError := kCandleIngestionService.marketDataProxy.FetchKCandles(
			executionContext, tradableChunk)
		if fetchError != nil {
			// Abandon the rest instead of hammering a refusing source into a ban; stored chunks stay and a rerun resumes from them.
			symbolReport.NoteFetchFailure(fetchError.Error())
			recordProgress(chunkIndex, symbolReport.ToDto())

			return nil
		}

		symbolReport.NoteAsked()

		judgedKCandles, skippedKCandles := kCandleIngestionService.judge(
			reportedKCandles, ingestionDomain)
		symbolReport.NoteSkipped(skippedKCandles...)

		storedCount, saveError := kCandleIngestionService.kCandleRepository.SaveAllIfAbsent(
			executionContext, judgedKCandles)
		if saveError != nil {
			// A storage failure is the system's fault, so it ends the run; earlier chunks are still reported as stored.
			recordProgress(chunkIndex, symbolReport.ToDto())

			return saveError
		}

		symbolReport.NoteStored(storedCount)
	}

	recordProgress(len(chunks), symbolReport.ToDto())

	return nil
}

// reachSymbolOnDemand validates and finds the symbol for the manual fetches and drops any presumed closure, since a manual request means the source should really be asked.
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

	// The market comes only from registration; it is deliberately never guessed from the symbol name.
	if !isRegistered {
		return entities.TradingSymbol{}, domains.KCandleIngestionDomain{}, fmt.Errorf("%w: %s",
			domains.ErrTradingSymbolNotRegistered, tradingSymbolDomain.Value())
	}

	kCandleIngestionService.marketClosureLedger.reconsider(
		kCandleIngestionService.marketCatalogDomain.MarketOf(registeredSymbol.Market).Value())

	return registeredSymbol, ingestionDomain, nil
}

// backfillWindowOf asks from the symbol's latest stored candle, bounded by the lookback.
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

// prepareRun reads the clock and watchlist once so the whole run shares one list and one "now"; rules are validated before storage is touched.
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

// buildIngestionDomain validates the rules against one clock reading before any watchlist or source is touched.
func (kCandleIngestionService *KCandleIngestionService) buildIngestionDomain() (
	domains.KCandleIngestionDomain, error,
) {
	return domains.NewKCandleIngestionDomain(
		kCandleIngestionService.clockProxy.Now(),
		kCandleIngestionService.roundCandleCount,
		kCandleIngestionService.backfillLookback,
	)
}

// ingestSymbols runs every symbol concurrently with a plain WaitGroup, since an errgroup would cancel the others on one failure; each goroutine writes only its own result slot.
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

// ingestSymbol fetches one symbol's window, clamped to its trading session so closed hours and presumed-closed days are simply empty windows; a source failure ends only this symbol.
func (kCandleIngestionService *KCandleIngestionService) ingestSymbol(
	executionContext context.Context,
	watchedSymbol entities.TradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	windowOf func(watchedSymbol entities.TradingSymbol, market vo.MarketVo) (vo.KCandleFetchWindowVo, error),
) dto.KCandleSymbolIngestionReportDto {
	marketDomain := kCandleIngestionService.marketCatalogDomain.MarketOf(watchedSymbol.Market)
	symbolReport := domains.NewKCandleSymbolIngestionReportDomain(
		watchedSymbol.Symbol, marketDomain.Value())

	if kCandleIngestionService.marketClosureLedger.isPresumedClosed(
		marketDomain.Value(), marketDomain.TradingDateOf(ingestionDomain.CurrentTime())) {
		return symbolReport.ToDto()
	}

	window, windowError := windowOf(watchedSymbol, marketDomain.Value())
	if windowError != nil {
		symbolReport.NoteFetchFailure(windowError.Error())

		return symbolReport.ToDto()
	}

	tradableWindow := marketDomain.ClampToTradingSession(window)
	if tradableWindow.IsEmpty() {
		return symbolReport.ToDto()
	}

	reportedKCandles, fetchError := kCandleIngestionService.marketDataProxy.FetchKCandles(
		executionContext, tradableWindow)
	if fetchError != nil {
		symbolReport.NoteFetchFailure(fetchError.Error())

		return symbolReport.ToDto()
	}

	symbolReport.NoteAsked()

	judgedKCandles, skippedKCandles := kCandleIngestionService.judge(
		reportedKCandles, ingestionDomain)
	symbolReport.NoteSkipped(skippedKCandles...)

	for _, judgedKCandle := range judgedKCandles {
		if _, saveError := kCandleIngestionService.kCandleRepository.Save(
			executionContext, judgedKCandle); saveError != nil {
			symbolReport.NoteSkipped(dto.SkippedKCandleDto{
				OpenTime: judgedKCandle.OpenTime.UTC(),
				Reason:   saveError.Error(),
			})
			continue
		}

		symbolReport.NoteStored(1)
	}

	return symbolReport.ToDto()
}

// judge validates source candles with the ordinary K candle rules, returning storable ones and the skipped ones with reasons.
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

// presumeClosedMarkets latches a market shut for the day when every asked symbol answered with no candles, so holidays need no maintained calendar; unreachable sources never count.
// Only overwriting runs may feed it, which is why the history sync bypasses ingestSymbols.
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
		// Round-the-clock markets have no holidays.
		if marketDomain.NeverCloses() {
			continue
		}

		// Silence counts only after the session has run at least as long as the round covers, otherwise one late candle after the open looks like a holiday.
		if marketDomain.SessionElapsedAt(currentTime) < roundCoverage {
			continue
		}

		kCandleIngestionService.marketClosureLedger.presumeClosed(
			market, marketDomain.TradingDateOf(currentTime))
	}
}

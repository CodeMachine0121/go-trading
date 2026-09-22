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

// ContractKCandleIngestionService keeps the stored perpetual contract K candles
// current without anyone asking. Its public use cases never call one another.
//
// **Every decision about which stretch to ask for is reused, not rewritten.** When to
// start, where a backfill resumes, how a long stretch is cut into days, which candles
// have actually closed — all of it comes from KCandleIngestionDomain, which knows
// nothing about the kind of candle being fetched. What is written here is only what is
// genuinely different: where the candles come from, where they go, and which rules
// judge them.
//
// It is simpler than the spot service by exactly one thing: perpetual contracts never
// close, so there is no holiday to presume, no session to clamp a window to, and no
// ledger of markets decided shut. A quiet stretch here is a quiet stretch and nothing
// more.
type ContractKCandleIngestionService struct {
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	kCandleContractHistorySyncRunRepository domaininterface.IKCandleContractHistorySyncRunRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMarketDataProxy                 domaininterface.IContractMarketDataProxy
	clockProxy                              domaininterface.IClockProxy
	// roundTheClockMarket is the calendar perpetual contracts keep. It is borrowed
	// from the catalogue rather than written out again because "how many one-minute
	// candles should this stretch hold" is already answered there, and a second copy
	// of that arithmetic is a second chance to get it wrong.
	roundTheClockMarket domains.MarketDomain
	roundCandleCount    int
	backfillLookback    time.Duration
}

func NewContractKCandleIngestionService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	kCandleContractHistorySyncRunRepository domaininterface.IKCandleContractHistorySyncRunRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	contractMarketDataProxy domaininterface.IContractMarketDataProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	roundCandleCount int,
	backfillLookback time.Duration,
) *ContractKCandleIngestionService {
	return &ContractKCandleIngestionService{
		kCandleContractRepository:               kCandleContractRepository,
		kCandleContractHistorySyncRunRepository: kCandleContractHistorySyncRunRepository,
		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMarketDataProxy:                 contractMarketDataProxy,
		clockProxy:                              clockProxy,
		roundTheClockMarket:                     marketCatalogDomain.MarketOf(string(vo.MarketCrypto)),
		roundCandleCount:                        roundCandleCount,
		backfillLookback:                        backfillLookback,
	}
}

// RunScheduledRound fetches the newest closed contract candles for every watched
// contract and stores them, replacing whatever was held for the same open time. It
// reports what happened rather than failing: one contract the source would not answer
// for must not take the others down with it.
func (contractKCandleIngestionService *ContractKCandleIngestionService) RunScheduledRound(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	watchedSymbols, ingestionDomain, prepareError := contractKCandleIngestionService.prepareRun(
		executionContext)
	if prepareError != nil {
		return dto.KCandleIngestionReportDto{}, prepareError
	}

	return contractKCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		func(watchedSymbol entities.ContractTradingSymbol) (vo.KCandleFetchWindowVo, error) {
			return ingestionDomain.ScheduledWindow(watchedSymbol.Symbol, vo.MarketCrypto), nil
		}), nil
}

// RunBackfill closes each watched contract's gap, reaching no further back than the
// lookback allows. A contract already up to date is left alone and its source is
// never called.
func (contractKCandleIngestionService *ContractKCandleIngestionService) RunBackfill(
	executionContext context.Context,
) (dto.KCandleIngestionReportDto, error) {
	watchedSymbols, ingestionDomain, prepareError := contractKCandleIngestionService.prepareRun(
		executionContext)
	if prepareError != nil {
		return dto.KCandleIngestionReportDto{}, prepareError
	}

	return contractKCandleIngestionService.ingestSymbols(executionContext, watchedSymbols, ingestionDomain,
		contractKCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain)), nil
}

// RunBackfillFor closes one contract's gap on demand, reaching no further back than
// the lookback allows.
//
// It reaches the contract by name rather than through the watchlist, because a chart
// can be opened for a contract nobody is watching, and catching that one up is exactly
// what somebody looking at it is asking for.
func (contractKCandleIngestionService *ContractKCandleIngestionService) RunBackfillFor(
	executionContext context.Context, symbol string,
) (dto.KCandleIngestionReportDto, error) {
	registeredSymbol, ingestionDomain, reachError := contractKCandleIngestionService.
		reachSymbolOnDemand(executionContext, symbol)
	if reachError != nil {
		return dto.KCandleIngestionReportDto{}, reachError
	}

	return contractKCandleIngestionService.ingestSymbols(
		executionContext,
		[]entities.ContractTradingSymbol{registeredSymbol},
		ingestionDomain,
		contractKCandleIngestionService.backfillWindowOf(executionContext, ingestionDomain),
	), nil
}

// StartHistorySyncFor accepts a request to fill in the minutes missing from a stretch
// of one contract's history, and answers with where to watch it happen.
//
// **It answers before the fetching starts.** A stretch of years is thousands of paced
// requests — and twice as many here as on the spot side, because every minute takes
// two questions — so there is no connection worth holding open for it. The run is
// written down first, the work is driven by something that outlives the request, and
// the caller is handed the run to come back and look at.
//
// Everything that can refuse the request still refuses it here, before anything is
// recorded.
func (contractKCandleIngestionService *ContractKCandleIngestionService) StartHistorySyncFor(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, ceilingDays int,
) (dto.KCandleHistorySyncRunDto, error) {
	lookback, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		syncDto.LookbackDays, ceilingDays)
	if lookbackError != nil {
		return dto.KCandleHistorySyncRunDto{}, lookbackError
	}

	registeredSymbol, ingestionDomain, reachError := contractKCandleIngestionService.
		reachSymbolOnDemand(executionContext, syncDto.Symbol)
	if reachError != nil {
		return dto.KCandleHistorySyncRunDto{}, reachError
	}

	chunks := ingestionDomain.HistoryChunks(
		registeredSymbol.Symbol, vo.MarketCrypto, lookback.Duration())

	syncRun, saveError := contractKCandleIngestionService.kCandleContractHistorySyncRunRepository.
		Save(executionContext, entities.KCandleContractHistorySyncRun{
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

	go (&contractKCandleHistorySyncRunner{
		contractKCandleIngestionService: contractKCandleIngestionService,
		syncRun:                         syncRun,
		registeredSymbol:                registeredSymbol,
		ingestionDomain:                 ingestionDomain,
		chunks:                          chunks,
	}).run()

	return syncRun.ToDto(), nil
}

// GetHistorySyncRun answers with where one contract history sync has got to.
func (contractKCandleIngestionService *ContractKCandleIngestionService) GetHistorySyncRun(
	executionContext context.Context, id uint,
) (dto.KCandleHistorySyncRunDto, error) {
	syncRun, found, findError := contractKCandleIngestionService.
		kCandleContractHistorySyncRunRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.KCandleHistorySyncRunDto{}, findError
	}
	if !found {
		return dto.KCandleHistorySyncRunDto{}, ErrKCandleHistorySyncRunNotFound
	}

	return syncRun.ToDto(), nil
}

// FailInterruptedHistorySyncs clears out the contract runs the last shutdown cut off,
// and says how many there were.
func (contractKCandleIngestionService *ContractKCandleIngestionService) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return contractKCandleIngestionService.kCandleContractHistorySyncRunRepository.FailAllRunning(
		executionContext, kCandleHistorySyncInterrupted,
		contractKCandleIngestionService.clockProxy.Now())
}

// syncSymbolHistory walks one contract's stretch a chunk at a time, asking the source
// for a chunk and storing it before moving to the next.
//
// **A chunk already complete is not asked about**, counted against the round-the-clock
// calendar — which for a perpetual contract is every minute there is.
//
// **A stretch the venue has no mark price for is never complete by that measure**, so
// it is re-fetched by every sync that covers it. This is not hypothetical: a
// contract's mark price history begins later than its candle history, so the oldest
// months of any long sync are exactly the ones that keep being asked about. What it
// costs is requests, not correctness — those minutes are unstorable either way, and
// the run's skipped count says so out loud. Making the shortcut cover them means
// remembering which minutes are unfillable, which is a record this system does not
// keep yet.
//
// It looks at what is stored only to decide whether to ask, never to decide where to
// start. Starting from what is stored is what leaves a hole in the middle unreachable,
// which is the whole reason this use case exists.
func (contractKCandleIngestionService *ContractKCandleIngestionService) syncSymbolHistory(
	executionContext context.Context,
	registeredSymbol entities.ContractTradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	chunks []vo.KCandleFetchWindowVo,
	recordProgress func(completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto),
) error {
	symbolReport := contractKCandleIngestionService.newReportFor(registeredSymbol.Symbol)

	// Everything this walk has to say travels back through recordProgress rather than
	// through the return, so that whatever it managed before it stopped is already
	// written down.
	for chunkIndex, chunk := range chunks {
		recordProgress(chunkIndex, symbolReport.ToDto())

		alreadyHeld, countError := contractKCandleIngestionService.kCandleContractRepository.
			CountInRange(executionContext, registeredSymbol.Symbol, chunk.StartTime, chunk.EndTime)
		if countError != nil {
			recordProgress(chunkIndex, symbolReport.ToDto())

			return countError
		}

		if alreadyHeld >= contractKCandleIngestionService.roundTheClockMarket.
			TradingKCandleCountBetween(chunk.StartTime, chunk.EndTime) {
			continue
		}

		reportedCandles, fetchError := contractKCandleIngestionService.contractMarketDataProxy.
			FetchKCandles(executionContext, chunk)
		if fetchError != nil {
			// The rest of the stretch is abandoned rather than attempted. A source
			// that just refused one chunk will refuse the next two thousand the same
			// way, and hammering it is how a rate limit becomes a ban.
			symbolReport.NoteFetchFailure(fetchError.Error())
			recordProgress(chunkIndex, symbolReport.ToDto())

			return nil
		}

		symbolReport.NoteAsked()

		judgedCandles, skippedCandles := contractKCandleIngestionService.judge(
			reportedCandles, ingestionDomain)
		symbolReport.NoteSkipped(skippedCandles...)

		storedCount, saveError := contractKCandleIngestionService.kCandleContractRepository.
			SaveAllIfAbsent(executionContext, judgedCandles)
		if saveError != nil {
			recordProgress(chunkIndex, symbolReport.ToDto())

			return saveError
		}

		symbolReport.NoteStored(storedCount)
	}

	recordProgress(len(chunks), symbolReport.ToDto())

	return nil
}

// reachSymbolOnDemand is everything the two hand-driven fetches do before they differ:
// judge the name, settle the rules, and find the registration.
func (contractKCandleIngestionService *ContractKCandleIngestionService) reachSymbolOnDemand(
	executionContext context.Context, symbol string,
) (entities.ContractTradingSymbol, domains.KCandleIngestionDomain, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return entities.ContractTradingSymbol{}, domains.KCandleIngestionDomain{},
			fmt.Errorf("%w: %w", domains.ErrTradingSymbolNamed, symbolError)
	}

	ingestionDomain, buildError := contractKCandleIngestionService.buildIngestionDomain()
	if buildError != nil {
		return entities.ContractTradingSymbol{}, domains.KCandleIngestionDomain{}, buildError
	}

	registeredSymbol, isRegistered, findError := contractKCandleIngestionService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return entities.ContractTradingSymbol{}, domains.KCandleIngestionDomain{}, findError
	}

	if !isRegistered {
		return entities.ContractTradingSymbol{}, domains.KCandleIngestionDomain{},
			fmt.Errorf("%w: %s", domains.ErrTradingSymbolNotRegistered, contractSymbol.Value())
	}

	return registeredSymbol, ingestionDomain, nil
}

// backfillWindowOf is how far back one contract has to be asked about: from wherever
// its own history left off, and no further back than the lookback allows.
func (contractKCandleIngestionService *ContractKCandleIngestionService) backfillWindowOf(
	executionContext context.Context, ingestionDomain domains.KCandleIngestionDomain,
) func(entities.ContractTradingSymbol) (vo.KCandleFetchWindowVo, error) {
	return func(watchedSymbol entities.ContractTradingSymbol) (vo.KCandleFetchWindowVo, error) {
		latestStored, findError := contractKCandleIngestionService.kCandleContractRepository.
			FindLatest(executionContext, watchedSymbol.Symbol, 1)
		if findError != nil {
			return vo.KCandleFetchWindowVo{}, findError
		}

		if len(latestStored) == 0 {
			return ingestionDomain.BackfillWindow(
				watchedSymbol.Symbol, vo.MarketCrypto, time.Time{}), nil
		}

		return ingestionDomain.BackfillWindow(
			watchedSymbol.Symbol, vo.MarketCrypto, latestStored[0].OpenTime), nil
	}
}

// prepareRun reads the clock and the watchlist once each, so that every window in one
// run and every candle judged during it share one list and one idea of "now".
func (contractKCandleIngestionService *ContractKCandleIngestionService) prepareRun(
	executionContext context.Context,
) ([]entities.ContractTradingSymbol, domains.KCandleIngestionDomain, error) {
	ingestionDomain, buildError := contractKCandleIngestionService.buildIngestionDomain()
	if buildError != nil {
		return nil, domains.KCandleIngestionDomain{}, buildError
	}

	watchedSymbols, findError := contractKCandleIngestionService.contractTradingSymbolRepository.
		FindWatched(executionContext)
	if findError != nil {
		return nil, domains.KCandleIngestionDomain{}, findError
	}

	return watchedSymbols, ingestionDomain, nil
}

// buildIngestionDomain settles the rules against one reading of the clock.
func (contractKCandleIngestionService *ContractKCandleIngestionService) buildIngestionDomain() (
	domains.KCandleIngestionDomain, error,
) {
	return domains.NewKCandleIngestionDomain(
		contractKCandleIngestionService.clockProxy.Now(),
		contractKCandleIngestionService.roundCandleCount,
		contractKCandleIngestionService.backfillLookback,
	)
}

// ingestSymbols runs every watched contract at once. A plain wait group is deliberate:
// an error group would cancel the remaining contracts the moment one failed, which is
// the opposite of what independence per symbol means here.
func (contractKCandleIngestionService *ContractKCandleIngestionService) ingestSymbols(
	executionContext context.Context,
	watchedSymbols []entities.ContractTradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	windowOf func(watchedSymbol entities.ContractTradingSymbol) (vo.KCandleFetchWindowVo, error),
) dto.KCandleIngestionReportDto {
	symbolReports := make([]dto.KCandleSymbolIngestionReportDto, len(watchedSymbols))

	var waitGroup sync.WaitGroup
	for index, watchedSymbol := range watchedSymbols {
		waitGroup.Go(func() {
			symbolReports[index] = contractKCandleIngestionService.ingestSymbol(
				executionContext, watchedSymbol, ingestionDomain, windowOf)
		})
	}
	waitGroup.Wait()

	return dto.KCandleIngestionReportDto{SymbolReports: symbolReports}
}

// ingestSymbol carries one contract from its window to what was stored. A source that
// will not answer ends this contract; a single candle that breaks a rule only ends
// itself — and a candle whose mark price did not arrive breaks a rule.
func (contractKCandleIngestionService *ContractKCandleIngestionService) ingestSymbol(
	executionContext context.Context,
	watchedSymbol entities.ContractTradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	windowOf func(watchedSymbol entities.ContractTradingSymbol) (vo.KCandleFetchWindowVo, error),
) dto.KCandleSymbolIngestionReportDto {
	symbolReport := contractKCandleIngestionService.newReportFor(watchedSymbol.Symbol)

	window, windowError := windowOf(watchedSymbol)
	if windowError != nil {
		symbolReport.NoteFetchFailure(windowError.Error())

		return symbolReport.ToDto()
	}

	// The window is not narrowed to a trading session, because a perpetual contract
	// has none: every minute of it is market.
	if window.IsEmpty() {
		return symbolReport.ToDto()
	}

	reportedCandles, fetchError := contractKCandleIngestionService.contractMarketDataProxy.
		FetchKCandles(executionContext, window)
	if fetchError != nil {
		symbolReport.NoteFetchFailure(fetchError.Error())

		return symbolReport.ToDto()
	}

	symbolReport.NoteAsked()

	judgedCandles, skippedCandles := contractKCandleIngestionService.judge(
		reportedCandles, ingestionDomain)
	symbolReport.NoteSkipped(skippedCandles...)

	for _, judgedCandle := range judgedCandles {
		if _, saveError := contractKCandleIngestionService.kCandleContractRepository.Save(
			executionContext, judgedCandle); saveError != nil {
			symbolReport.NoteSkipped(dto.SkippedKCandleDto{
				OpenTime: judgedCandle.OpenTime.UTC(),
				Reason:   saveError.Error(),
			})
			continue
		}

		symbolReport.NoteStored(1)
	}

	return symbolReport.ToDto()
}

// newReportFor starts one contract's report.
//
// The market is named as the round-the-clock one because that is the calendar these
// contracts keep, and a report with the field left blank would read as a market
// nobody recognised.
func (contractKCandleIngestionService *ContractKCandleIngestionService) newReportFor(
	symbol string,
) *domains.KCandleSymbolIngestionReportDomain {
	return domains.NewKCandleSymbolIngestionReportDomain(
		symbol, contractKCandleIngestionService.roundTheClockMarket.Value())
}

// judge puts everything the source answered with through the contract K candle rules,
// handing back the ones that may be stored and naming the ones that may not.
//
// **A missing mark price is judged here and nowhere else.** It arrives as an absent
// figure and leaves as a skipped candle with the reason attached, which is the same
// path a candle whose high sat below its low takes. It needed no branch of its own,
// and that is the point of letting the absence survive the trip from the source.
func (contractKCandleIngestionService *ContractKCandleIngestionService) judge(
	reportedCandles []vo.ContractMarketKCandleVo, ingestionDomain domains.KCandleIngestionDomain,
) ([]entities.KCandleContract, []dto.SkippedKCandleDto) {
	latestClosedOpenTime := ingestionDomain.LatestClosedOpenTime()

	judgedCandles := make([]entities.KCandleContract, 0, len(reportedCandles))
	skippedCandles := make([]dto.SkippedKCandleDto, 0)

	for _, reportedCandle := range reportedCandles {
		// The minute still running is not stored, by either half. Its figures are
		// still moving, and a mark price for it would be just as provisional.
		if reportedCandle.OpenTime.After(latestClosedOpenTime) {
			continue
		}

		contractDomain, validationError := domains.NewKCandleContractDomain(
			reportedCandle.ToWriteDto(), ingestionDomain.CurrentTime())
		if validationError != nil {
			skippedCandles = append(skippedCandles, dto.SkippedKCandleDto{
				OpenTime: reportedCandle.OpenTime.UTC(),
				Reason:   validationError.Error(),
			})
			continue
		}

		judgedCandles = append(judgedCandles, contractDomain.ToEntity())
	}

	return judgedCandles, skippedCandles
}

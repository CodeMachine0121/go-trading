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

// ContractKCandleIngestionService keeps stored contract K candles current, reusing KCandleIngestionDomain's window logic; contracts never close, so there are no holidays or sessions.
type ContractKCandleIngestionService struct {
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	kCandleContractHistorySyncRunRepository domaininterface.IKCandleContractHistorySyncRunRepository
	contractTradingSymbolRepository         domaininterface.IContractTradingSymbolRepository
	contractMarketDataProxy                 domaininterface.IContractMarketDataProxy
	clockProxy                              domaininterface.IClockProxy
	// roundTheClockMarket is borrowed from the catalogue to reuse its expected-candle-count arithmetic.
	roundTheClockMarket domains.MarketDomain
	roundCandleCount    int
	backfillLookback    time.Duration
	// positionStatisticService fills the position statistics half of each history sync.
	positionStatisticService *ContractPositionStatisticService
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
	positionStatisticService *ContractPositionStatisticService,
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
		positionStatisticService:                positionStatisticService,
	}
}

// RunScheduledRound fetches and upserts the newest closed candles for every watched contract, reporting per-contract failures instead of returning an error.
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

// RunBackfill closes each watched contract's gap within the lookback; up-to-date contracts are not fetched.
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

// RunBackfillFor closes one contract's gap on demand; it looks the contract up by name because charts can open unwatched contracts.
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

// StartHistorySyncFor validates and records a history sync (candles, then position statistics) and returns the run before fetching starts, since years of paced requests cannot hold a connection.
func (contractKCandleIngestionService *ContractKCandleIngestionService) StartHistorySyncFor(
	executionContext context.Context, syncDto dto.KCandleHistorySyncDto, ceilingDays int,
) (dto.KCandleContractHistorySyncRunDto, error) {
	lookback, lookbackError := domains.NewKCandleHistoryLookbackDomain(
		syncDto.LookbackDays, ceilingDays)
	if lookbackError != nil {
		return dto.KCandleContractHistorySyncRunDto{}, lookbackError
	}

	registeredSymbol, ingestionDomain, reachError := contractKCandleIngestionService.
		reachSymbolOnDemand(executionContext, syncDto.Symbol)
	if reachError != nil {
		return dto.KCandleContractHistorySyncRunDto{}, reachError
	}

	chunks := ingestionDomain.HistoryChunks(
		registeredSymbol.Symbol, vo.MarketCrypto, lookback.Duration())
	// Statistics use the same clock reading and lookback so both histories cover the same stretch.
	positionStatisticHistory := domains.NewContractPositionStatisticHistoryDomain(
		ingestionDomain.CurrentTime(), lookback.Duration())

	syncRun, saveError := contractKCandleIngestionService.kCandleContractHistorySyncRunRepository.
		Save(executionContext, entities.KCandleContractHistorySyncRun{
			Symbol:                     registeredSymbol.Symbol,
			LookbackDays:               syncDto.LookbackDays,
			Status:                     string(vo.KCandleHistorySyncRunning),
			TotalChunks:                len(chunks),
			StartedAt:                  ingestionDomain.CurrentTime(),
			PositionStatisticTotalDays: len(positionStatisticHistory.Days()),
		})
	if saveError != nil {
		// Do not start work that no one could find or a restart could sweep.
		return dto.KCandleContractHistorySyncRunDto{}, saveError
	}

	go (&contractKCandleHistorySyncRunner{
		contractKCandleIngestionService: contractKCandleIngestionService,
		syncRun:                         syncRun,
		registeredSymbol:                registeredSymbol,
		ingestionDomain:                 ingestionDomain,
		chunks:                          chunks,
		positionStatisticHistory:        positionStatisticHistory,
		positionStatisticProgress: dto.ContractPositionStatisticSyncProgressDto{
			TotalDays: len(positionStatisticHistory.Days()),
		},
	}).run()

	return syncRun.ToDto(), nil
}

func (contractKCandleIngestionService *ContractKCandleIngestionService) GetHistorySyncRun(
	executionContext context.Context, id uint,
) (dto.KCandleContractHistorySyncRunDto, error) {
	syncRun, found, findError := contractKCandleIngestionService.
		kCandleContractHistorySyncRunRepository.FindOne(executionContext, id)
	if findError != nil {
		return dto.KCandleContractHistorySyncRunDto{}, findError
	}
	if !found {
		return dto.KCandleContractHistorySyncRunDto{}, ErrKCandleHistorySyncRunNotFound
	}

	return syncRun.ToDto(), nil
}

// FailInterruptedHistorySyncs fails the contract runs cut off by the last shutdown and returns how many.
func (contractKCandleIngestionService *ContractKCandleIngestionService) FailInterruptedHistorySyncs(
	executionContext context.Context,
) (int, error) {
	return contractKCandleIngestionService.kCandleContractHistorySyncRunRepository.FailAllRunning(
		executionContext, kCandleHistorySyncInterrupted,
		contractKCandleIngestionService.clockProxy.Now())
}

// syncSymbolHistory walks the stretch chunk by chunk, skipping chunks already complete; stretches without mark prices are never complete and so are always refetched (costing requests, not correctness).
func (contractKCandleIngestionService *ContractKCandleIngestionService) syncSymbolHistory(
	executionContext context.Context,
	registeredSymbol entities.ContractTradingSymbol,
	ingestionDomain domains.KCandleIngestionDomain,
	chunks []vo.KCandleFetchWindowVo,
	recordProgress func(completedChunks int, symbolReport dto.KCandleSymbolIngestionReportDto),
) error {
	symbolReport := contractKCandleIngestionService.newReportFor(registeredSymbol.Symbol)

	// Progress is reported through recordProgress so partial work is already recorded if the walk stops.
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
			// Abandon the rest: a refusing source will keep refusing, and hammering it risks a ban.
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

// reachSymbolOnDemand validates the name, settles the rules and finds the registration for the on-demand fetches.
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

// backfillWindowOf starts where the contract's history left off, capped by the lookback.
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

// prepareRun reads the clock and watchlist once so the whole run shares them.
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

func (contractKCandleIngestionService *ContractKCandleIngestionService) buildIngestionDomain() (
	domains.KCandleIngestionDomain, error,
) {
	return domains.NewKCandleIngestionDomain(
		contractKCandleIngestionService.clockProxy.Now(),
		contractKCandleIngestionService.roundCandleCount,
		contractKCandleIngestionService.backfillLookback,
	)
}

// ingestSymbols uses a plain WaitGroup, not errgroup, so one failing contract does not cancel the rest.
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

// ingestSymbol ends on a source error, while a rule-breaking candle (including a missing mark price) only skips itself.
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

	// No session clamping: a perpetual contract trades every minute.
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

// newReportFor names the round-the-clock market so the report never shows a blank market.
func (contractKCandleIngestionService *ContractKCandleIngestionService) newReportFor(
	symbol string,
) *domains.KCandleSymbolIngestionReportDomain {
	return domains.NewKCandleSymbolIngestionReportDomain(
		symbol, contractKCandleIngestionService.roundTheClockMarket.Value())
}

// judge applies the contract K candle rules, which is the only place a missing mark price turns into a skipped candle.
func (contractKCandleIngestionService *ContractKCandleIngestionService) judge(
	reportedCandles []vo.ContractMarketKCandleVo, ingestionDomain domains.KCandleIngestionDomain,
) ([]entities.KCandleContract, []dto.SkippedKCandleDto) {
	latestClosedOpenTime := ingestionDomain.LatestClosedOpenTime()

	judgedCandles := make([]entities.KCandleContract, 0, len(reportedCandles))
	skippedCandles := make([]dto.SkippedKCandleDto, 0)

	for _, reportedCandle := range reportedCandles {
		// The still-running minute is not stored; its figures, mark price included, are provisional.
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

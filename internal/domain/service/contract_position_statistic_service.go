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
)

// ContractPositionStatisticService keeps recording the five-minute position
// statistics of every watched perpetual contract, and answers questions about them.
// Its public use cases never call one another.
//
// **Recording is the whole point.** The venue keeps only the last thirty days, so a
// stretch this system did not record while it could is a stretch nobody will ever
// have. Every trigger — starting up, a contract joining the watchlist, the round every
// five minutes — asks the same thing: everything since an hour before the last
// statistic held, and no further back than the venue can still answer for.
type ContractPositionStatisticService struct {
	statisticRepository             domaininterface.IContractPositionStatisticRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	positionStatisticProxy          domaininterface.IContractPositionStatisticProxy
	// positionStatisticArchiveProxy is the venue's history archive, which keeps the
	// same statistics for years where the live source keeps thirty days. Only a
	// contract history sync reads it; every round above still asks the live source.
	positionStatisticArchiveProxy domaininterface.IContractPositionStatisticArchiveProxy
	clockProxy                    domaininterface.IClockProxy
	queryMaxResults               int
}

func NewContractPositionStatisticService(
	statisticRepository domaininterface.IContractPositionStatisticRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	positionStatisticProxy domaininterface.IContractPositionStatisticProxy,
	positionStatisticArchiveProxy domaininterface.IContractPositionStatisticArchiveProxy,
	clockProxy domaininterface.IClockProxy,
	queryMaxResults int,
) *ContractPositionStatisticService {
	return &ContractPositionStatisticService{
		statisticRepository:             statisticRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		positionStatisticProxy:          positionStatisticProxy,
		positionStatisticArchiveProxy:   positionStatisticArchiveProxy,
		clockProxy:                      clockProxy,
		queryMaxResults:                 queryMaxResults,
	}
}

// RunRound records whatever is new for every watched contract. It reports what
// happened rather than failing: one contract the venue would not answer for must not
// take the others with it, and the next round picks each one up where it stopped.
func (contractPositionStatisticService *ContractPositionStatisticService) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	currentTime := contractPositionStatisticService.clockProxy.Now()

	watchedSymbols, findError := contractPositionStatisticService.contractTradingSymbolRepository.
		FindWatched(executionContext)
	if findError != nil {
		return dto.ContractSeriesIngestionReportDto{}, findError
	}

	symbolReports := make([]dto.ContractSeriesSymbolReportDto, len(watchedSymbols))

	// A plain wait group rather than an error group, for the reason the funding rate
	// round gives: independence per contract means one failure cancels nothing.
	var waitGroup sync.WaitGroup
	for index, watchedSymbol := range watchedSymbols {
		waitGroup.Go(func() {
			symbolReports[index] = contractPositionStatisticService.recordSymbol(
				executionContext, watchedSymbol, currentTime)
		})
	}
	waitGroup.Wait()

	return dto.ContractSeriesIngestionReportDto{SymbolReports: symbolReports}, nil
}

// RunRoundFor records whatever is new for one registered contract — the one that has
// just joined the watchlist, whose last thirty days are waiting.
func (contractPositionStatisticService *ContractPositionStatisticService) RunRoundFor(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return dto.ContractSeriesSymbolReportDto{}, fmt.Errorf(
			"%w: %w", domains.ErrTradingSymbolNamed, symbolError)
	}

	registeredSymbol, isRegistered, findError := contractPositionStatisticService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return dto.ContractSeriesSymbolReportDto{}, findError
	}
	if !isRegistered {
		return dto.ContractSeriesSymbolReportDto{}, fmt.Errorf(
			"%w: %s", domains.ErrTradingSymbolNotRegistered, contractSymbol.Value())
	}

	return contractPositionStatisticService.recordSymbol(
		executionContext, registeredSymbol, contractPositionStatisticService.clockProxy.Now()), nil
}

// FindStatisticsInRange returns the statistics of one contract whose statistic time
// falls inside the range, earliest first. A range holding more than the configured
// maximum is refused rather than cut short.
func (contractPositionStatisticService *ContractPositionStatisticService) FindStatisticsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractPositionStatisticDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		return nil, fmt.Errorf("%w: %w", domains.ErrContractPositionStatisticValidation, validationError)
	}

	statistics, findError := contractPositionStatisticService.statisticRepository.FindInRange(
		executionContext, queryDomain, contractPositionStatisticService.queryMaxResults+1)
	if findError != nil {
		return nil, findError
	}

	if len(statistics) > contractPositionStatisticService.queryMaxResults {
		return nil, fmt.Errorf("%w: 時間區間過大，請縮小區間（單次最多 %d 筆）",
			domains.ErrContractPositionStatisticValidation, contractPositionStatisticService.queryMaxResults)
	}

	statisticDtos := make([]dto.ContractPositionStatisticDto, 0, len(statistics))
	for _, statistic := range statistics {
		statisticDtos = append(statisticDtos, statistic.ToDto())
	}

	return statisticDtos, nil
}

// syncHistory walks one contract's position statistics a day at a time out of the
// venue's archive, storing only the ones not held yet. It is the second half of a
// contract history sync, driven by the same run as the candles; it is not a use case
// of its own, which is why nothing outside this package can reach it.
//
// **A day already whole is not asked about**, exactly as a candle chunk already whole
// is not. What is held is read only to decide whether to ask, never where to start.
//
// **A day the archive has no file for is simply passed**: not published yet, or the
// contract did not exist. The archive not answering — or answering with something that
// cannot be read — stops the statistics and is written down, but is not an error: the
// run found something out about the source rather than doing anything wrong. Only
// this system breaking is returned as one.
//
// Every archive reading is worked into the live source's shape and judged by the live
// rules, so a statistic from the archive and one recorded live are one kind of thing.
func (contractPositionStatisticService *ContractPositionStatisticService) syncHistory(
	executionContext context.Context,
	symbol string,
	historyDomain domains.ContractPositionStatisticHistoryDomain,
	recordProgress func(progress dto.ContractPositionStatisticSyncProgressDto),
) error {
	currentTime := contractPositionStatisticService.clockProxy.Now()
	days := historyDomain.Days()
	symbolReport := domains.NewContractSeriesSymbolReportDomain(symbol)
	progressAt := func(completedDays int) dto.ContractPositionStatisticSyncProgressDto {
		report := symbolReport.ToDto()

		return dto.ContractPositionStatisticSyncProgressDto{
			TotalDays:          len(days),
			CompletedDays:      completedDays,
			StoredCount:        report.StoredCount,
			SkippedCount:       report.SkippedCount,
			FetchFailureReason: report.FetchFailureReason,
		}
	}

	for dayIndex, day := range days {
		recordProgress(progressAt(dayIndex))

		heldCount, countError := contractPositionStatisticService.statisticRepository.CountInRange(
			executionContext, symbol, day.FirstStatisticTime, day.LastStatisticTime)
		if countError != nil {
			return countError
		}
		if historyDomain.IsDayComplete(heldCount) {
			continue
		}

		archivedStatistics, found, fetchError := contractPositionStatisticService.
			positionStatisticArchiveProxy.FetchDailyPositionStatistics(executionContext, symbol, day.Day)
		if fetchError != nil {
			// The rest of the days are abandoned rather than attempted: an archive that
			// just refused one day will refuse the next thousand the same way.
			symbolReport.NoteFetchFailure(fetchError.Error())
			recordProgress(progressAt(dayIndex))

			return nil
		}
		if !found {
			continue
		}

		judgedStatistics := make([]entities.ContractPositionStatistic, 0, len(archivedStatistics))
		for _, archivedStatistic := range archivedStatistics {
			archiveDomain, archiveError := domains.NewContractPositionStatisticArchiveDomain(archivedStatistic)
			if archiveError != nil {
				symbolReport.NoteSkipped(archivedStatistic.StatisticTime, archiveError.Error())
				continue
			}

			statisticDomain, validationError := domains.NewContractPositionStatisticDomain(
				archiveDomain.ToContractPositionStatisticVo(), currentTime)
			if validationError != nil {
				symbolReport.NoteSkipped(archivedStatistic.StatisticTime, validationError.Error())
				continue
			}
			judgedStatistics = append(judgedStatistics, statisticDomain.ToEntity())
		}

		storedCount, saveError := contractPositionStatisticService.statisticRepository.SaveAllIfAbsent(
			executionContext, judgedStatistics)
		if saveError != nil {
			return saveError
		}
		symbolReport.NoteStored(storedCount)
	}

	recordProgress(progressAt(len(days)))

	return nil
}

// recordSymbol carries one contract from its last held statistic to now. The venue or
// storage failing ends this contract's turn; one statistic breaking a rule — one of
// its splits missing among them — only ends itself, and since every round asks again
// over the last hour behind the latest one held, a gap left that way is asked about
// again.
func (contractPositionStatisticService *ContractPositionStatisticService) recordSymbol(
	executionContext context.Context,
	contractSymbol entities.ContractTradingSymbol,
	currentTime time.Time,
) dto.ContractSeriesSymbolReportDto {
	symbolReport := domains.NewContractSeriesSymbolReportDomain(contractSymbol.Symbol)

	latestStatistic, hasLatest, findError := contractPositionStatisticService.statisticRepository.
		FindLatest(executionContext, contractSymbol.Symbol)
	if findError != nil {
		symbolReport.NoteFetchFailure(findError.Error())

		return symbolReport.ToDto()
	}

	window := domains.NewContractPositionStatisticWindowDomain(
		currentTime, latestStatistic.StatisticTime, hasLatest)
	if window.IsEmpty() {
		return symbolReport.ToDto()
	}

	reportedStatistics, fetchError := contractPositionStatisticService.positionStatisticProxy.
		FetchPositionStatistics(executionContext, contractSymbol.Symbol, window.StartTime(), window.EndTime())
	if fetchError != nil {
		symbolReport.NoteFetchFailure(fetchError.Error())

		return symbolReport.ToDto()
	}

	judgedStatistics := make([]entities.ContractPositionStatistic, 0, len(reportedStatistics))
	for _, reportedStatistic := range reportedStatistics {
		statisticDomain, validationError := domains.NewContractPositionStatisticDomain(
			reportedStatistic, currentTime)
		if validationError != nil {
			symbolReport.NoteSkipped(reportedStatistic.StatisticTime, validationError.Error())
			continue
		}
		judgedStatistics = append(judgedStatistics, statisticDomain.ToEntity())
	}

	storedCount, saveError := contractPositionStatisticService.statisticRepository.SaveAllIfAbsent(
		executionContext, judgedStatistics)
	if saveError != nil {
		symbolReport.NoteFetchFailure(saveError.Error())

		return symbolReport.ToDto()
	}
	symbolReport.NoteStored(storedCount)

	return symbolReport.ToDto()
}

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

// ContractPositionStatisticService records every watched contract's five-minute position statistics, since the venue keeps only thirty days; every trigger fetches from an hour before the last held statistic.
type ContractPositionStatisticService struct {
	statisticRepository             domaininterface.IContractPositionStatisticRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	positionStatisticProxy          domaininterface.IContractPositionStatisticProxy
	// positionStatisticArchiveProxy reads the venue's multi-year archive; only contract history syncs use it.
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

// RunRound records new statistics for every watched contract, reporting per-contract failures instead of returning an error.
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

	// A plain WaitGroup, not errgroup, so one failing contract does not cancel the rest.
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

// RunRoundFor records new statistics for one contract, e.g. one that just joined the watchlist.
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

// FindStatisticsInRange returns one contract's statistics in range, earliest first; a range over the maximum is refused rather than truncated.
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

// syncHistory is the statistics half of a contract history sync: it walks the archive day by day, skipping complete days and missing files; an archive refusal is recorded rather than returned, and only internal failures are errors.
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
			// Abandon the remaining days: an archive that refused one will refuse the rest.
			symbolReport.NoteFetchFailure(fetchError.Error())
			recordProgress(progressAt(dayIndex))

			return nil
		}
		if !found {
			continue
		}

		judgedStatistics := make([]entities.ContractPositionStatistic, 0, len(archivedStatistics))
		for _, archivedStatistic := range archivedStatistics {
			archiveDomain, validationError := domains.NewContractPositionStatisticArchiveDomain(
				archivedStatistic, currentTime)
			if validationError != nil {
				symbolReport.NoteSkipped(archivedStatistic.StatisticTime, validationError.Error())
				continue
			}
			judgedStatistics = append(judgedStatistics, archiveDomain.ToEntity())
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

// recordSymbol brings one contract up to date; venue or storage errors end its turn, while a rule-breaking statistic only skips itself and is retried by the next round's one-hour overlap.
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

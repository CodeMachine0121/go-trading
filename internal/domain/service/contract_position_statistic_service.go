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
	clockProxy                      domaininterface.IClockProxy
	queryMaxResults                 int
}

func NewContractPositionStatisticService(
	statisticRepository domaininterface.IContractPositionStatisticRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	positionStatisticProxy domaininterface.IContractPositionStatisticProxy,
	clockProxy domaininterface.IClockProxy,
	queryMaxResults int,
) *ContractPositionStatisticService {
	return &ContractPositionStatisticService{
		statisticRepository:             statisticRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		positionStatisticProxy:          positionStatisticProxy,
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

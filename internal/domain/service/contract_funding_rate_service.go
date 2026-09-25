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

// ContractFundingRateService keeps every watched contract's funding settlements complete via one catch-up path (after the last held settlement, or from the first) used by every trigger.
type ContractFundingRateService struct {
	settlementRepository            domaininterface.IContractFundingRateSettlementRepository
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository
	fundingRateProxy                domaininterface.IContractFundingRateProxy
	clockProxy                      domaininterface.IClockProxy
	queryMaxResults                 int
}

func NewContractFundingRateService(
	settlementRepository domaininterface.IContractFundingRateSettlementRepository,
	contractTradingSymbolRepository domaininterface.IContractTradingSymbolRepository,
	fundingRateProxy domaininterface.IContractFundingRateProxy,
	clockProxy domaininterface.IClockProxy,
	queryMaxResults int,
) *ContractFundingRateService {
	return &ContractFundingRateService{
		settlementRepository:            settlementRepository,
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		fundingRateProxy:                fundingRateProxy,
		clockProxy:                      clockProxy,
		queryMaxResults:                 queryMaxResults,
	}
}

// RunRound catches every watched contract up, reporting per-contract failures instead of returning an error.
func (contractFundingRateService *ContractFundingRateService) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	currentTime := contractFundingRateService.clockProxy.Now()

	watchedSymbols, findError := contractFundingRateService.contractTradingSymbolRepository.
		FindWatched(executionContext)
	if findError != nil {
		return dto.ContractSeriesIngestionReportDto{}, findError
	}

	symbolReports := make([]dto.ContractSeriesSymbolReportDto, len(watchedSymbols))

	// A plain WaitGroup, not errgroup, so one failing contract does not cancel the rest.
	var waitGroup sync.WaitGroup
	for index, watchedSymbol := range watchedSymbols {
		waitGroup.Go(func() {
			symbolReports[index] = contractFundingRateService.catchUpSymbol(
				executionContext, watchedSymbol, currentTime)
		})
	}
	waitGroup.Wait()

	return dto.ContractSeriesIngestionReportDto{SymbolReports: symbolReports}, nil
}

// RunRoundFor catches one contract up on demand, e.g. when it joins the watchlist.
func (contractFundingRateService *ContractFundingRateService) RunRoundFor(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return dto.ContractSeriesSymbolReportDto{}, fmt.Errorf(
			"%w: %w", domains.ErrTradingSymbolNamed, symbolError)
	}

	registeredSymbol, isRegistered, findError := contractFundingRateService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return dto.ContractSeriesSymbolReportDto{}, findError
	}
	if !isRegistered {
		return dto.ContractSeriesSymbolReportDto{}, fmt.Errorf(
			"%w: %s", domains.ErrTradingSymbolNotRegistered, contractSymbol.Value())
	}

	return contractFundingRateService.catchUpSymbol(
		executionContext, registeredSymbol, contractFundingRateService.clockProxy.Now()), nil
}

// FindSettlementsInRange returns one contract's settlements in range, earliest first; a range over the maximum is refused rather than truncated.
func (contractFundingRateService *ContractFundingRateService) FindSettlementsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractFundingRateSettlementDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		// Re-badge the query model's K candle sentinel error as this path's own.
		return nil, fmt.Errorf("%w: %w",
			domains.ErrContractFundingRateSettlementValidation, validationError)
	}

	// Fetch one more than the maximum so "too many" shows without a separate count.
	settlements, findError := contractFundingRateService.settlementRepository.FindInRange(
		executionContext, queryDomain, contractFundingRateService.queryMaxResults+1)
	if findError != nil {
		return nil, findError
	}

	if len(settlements) > contractFundingRateService.queryMaxResults {
		return nil, fmt.Errorf("%w: 時間區間過大，請縮小區間（單次最多 %d 筆）",
			domains.ErrContractFundingRateSettlementValidation, contractFundingRateService.queryMaxResults)
	}

	settlementDtos := make([]dto.ContractFundingRateSettlementDto, 0, len(settlements))
	for _, settlement := range settlements {
		settlementDtos = append(settlementDtos, settlement.ToDto())
	}

	return settlementDtos, nil
}

// catchUpSymbol brings one contract up to date; venue or storage errors end its turn, while a rule-breaking settlement only skips itself.
func (contractFundingRateService *ContractFundingRateService) catchUpSymbol(
	executionContext context.Context,
	contractSymbol entities.ContractTradingSymbol,
	currentTime time.Time,
) dto.ContractSeriesSymbolReportDto {
	symbolReport := domains.NewContractSeriesSymbolReportDomain(contractSymbol.Symbol)

	latestSettlement, hasLatest, findError := contractFundingRateService.settlementRepository.
		FindLatest(executionContext, contractSymbol.Symbol)
	if findError != nil {
		symbolReport.NoteFetchFailure(findError.Error())

		return symbolReport.ToDto()
	}

	// A zero moment tells the proxy to start from the contract's first settlement.
	after := time.Time{}
	if hasLatest {
		after = latestSettlement.SettlementTime
	}

	reportedSettlements, fetchError := contractFundingRateService.fundingRateProxy.
		FetchFundingRateSettlements(executionContext, contractSymbol.Symbol, after)
	if fetchError != nil {
		symbolReport.NoteFetchFailure(fetchError.Error())

		return symbolReport.ToDto()
	}

	judgedSettlements := make([]entities.ContractFundingRateSettlement, 0, len(reportedSettlements))
	for _, reportedSettlement := range reportedSettlements {
		settlementDomain, validationError := domains.NewContractFundingRateSettlementDomain(
			reportedSettlement, currentTime)
		if validationError != nil {
			symbolReport.NoteSkipped(reportedSettlement.SettlementTime, validationError.Error())
			continue
		}
		judgedSettlements = append(judgedSettlements, settlementDomain.ToEntity())
	}

	storedCount, saveError := contractFundingRateService.settlementRepository.SaveAllIfAbsent(
		executionContext, judgedSettlements)
	if saveError != nil {
		symbolReport.NoteFetchFailure(saveError.Error())

		return symbolReport.ToDto()
	}
	symbolReport.NoteStored(storedCount)

	return symbolReport.ToDto()
}

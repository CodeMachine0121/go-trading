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

// ContractFundingRateService keeps the stored funding rate settlements of every
// watched perpetual contract complete, and answers questions about them. Its public
// use cases never call one another.
//
// **There is one way to catch a contract up, and every trigger uses it.** Starting
// up, a contract joining the watchlist and the hourly round all ask the same thing —
// every settlement after the last one held — and a contract with nothing held is
// asked about from its very first settlement. So there is no backfill window to keep
// in step with a scheduled one, and a contract that was down for a week and one that
// missed an hour are the same case.
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

// RunRound catches every watched contract up. It reports what happened rather than
// failing: a contract the venue would not answer for must not take the others with
// it, and the next round starts from wherever each one got to.
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

	// A plain wait group rather than an error group: an error group would cancel the
	// remaining contracts the moment one failed, which is the opposite of what
	// independence per contract means here.
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

// RunRoundFor catches one registered contract up on demand — the one that has just
// joined the watchlist.
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

// FindSettlementsInRange returns the settlements of one contract whose settlement
// time falls inside the range, earliest first. A range holding more than the
// configured maximum is refused rather than cut short.
func (contractFundingRateService *ContractFundingRateService) FindSettlementsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractFundingRateSettlementDto, error) {
	queryDomain, validationError := domains.NewKCandleQueryDomain(queryDto)
	if validationError != nil {
		// Re-badged before it leaves: the query model answers in the K candle
		// sentinel, and a caller of this path recognises this path's.
		return nil, fmt.Errorf("%w: %w",
			domains.ErrContractFundingRateSettlementValidation, validationError)
	}

	// One more than the maximum is asked for, so that "too many" is something the
	// answer shows rather than something a second count has to establish.
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

// catchUpSymbol carries one contract from its last held settlement to now. The venue
// or storage failing ends this contract's turn; one settlement breaking a rule only
// ends itself.
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

	// Nothing held means from the contract's first settlement, which the proxy
	// reads a zero moment as.
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

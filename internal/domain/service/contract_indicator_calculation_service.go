package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractIndicatorCalculationService is the application layer's only entry point for running indicator scripts over contract bars; request rules are the spot calculation's, with a round-the-clock calendar.
type ContractIndicatorCalculationService struct {
	kCandleContractRepository               domaininterface.IKCandleContractRepository
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	contractPositionStatisticRepository     domaininterface.IContractPositionStatisticRepository
	contractIndicatorScriptProxy            domaininterface.IContractIndicatorScriptProxy
	clockProxy                              domaininterface.IClockProxy
	roundTheClockMarket                     domains.MarketDomain
	maxCandleCount                          int
}

func NewContractIndicatorCalculationService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	contractPositionStatisticRepository domaininterface.IContractPositionStatisticRepository,
	contractIndicatorScriptProxy domaininterface.IContractIndicatorScriptProxy,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	maxCandleCount int,
) *ContractIndicatorCalculationService {
	return &ContractIndicatorCalculationService{
		kCandleContractRepository:               kCandleContractRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		contractPositionStatisticRepository:     contractPositionStatisticRepository,
		contractIndicatorScriptProxy:            contractIndicatorScriptProxy,
		clockProxy:                              clockProxy,
		roundTheClockMarket:                     marketCatalogDomain.MarketOf(string(vo.MarketCrypto)),
		maxCandleCount:                          maxCandleCount,
	}
}

// CalculateContractIndicator runs the script over the requested number of finished contract bars using four stored reads; nothing is fetched to fill gaps.
func (contractIndicatorCalculationService *ContractIndicatorCalculationService) CalculateContractIndicator(
	executionContext context.Context, requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto,
		contractIndicatorCalculationService.roundTheClockMarket,
		contractIndicatorCalculationService.maxCandleCount,
		contractIndicatorCalculationService.clockProxy.Now(),
	)
	if validationError != nil {
		return dto.IndicatorCalculationResultDto{}, validationError
	}

	newestFirstKCandleContracts, findCandlesError := contractIndicatorCalculationService.kCandleContractRepository.
		FindLatestBefore(
			executionContext,
			calculationDomain.Symbol(),
			calculationDomain.ReadCutoff(),
			calculationDomain.SourceCandleLimit(),
		)
	if findCandlesError != nil {
		return dto.IndicatorCalculationResultDto{}, findCandlesError
	}

	alignmentDomain, selectionError := calculationDomain.SelectContractInput(newestFirstKCandleContracts)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	settlements, findSettlementsError := contractIndicatorCalculationService.contractFundingRateSettlementRepository.
		FindInRange(executionContext, alignmentDomain.SettlementQuery(), alignmentDomain.SettlementReadLimit())
	if findSettlementsError != nil {
		return dto.IndicatorCalculationResultDto{}, findSettlementsError
	}

	leadInSettlement, hasLeadIn, findLeadInError := contractIndicatorCalculationService.
		contractFundingRateSettlementRepository.
		FindLatestBefore(executionContext, calculationDomain.Symbol(), alignmentDomain.SettlementLeadInCutoff())
	if findLeadInError != nil {
		return dto.IndicatorCalculationResultDto{}, findLeadInError
	}
	if hasLeadIn {
		settlements = append(settlements, leadInSettlement)
	}

	statistics, findStatisticsError := contractIndicatorCalculationService.contractPositionStatisticRepository.
		FindInRange(executionContext, alignmentDomain.StatisticQuery(), alignmentDomain.StatisticReadLimit())
	if findStatisticsError != nil {
		return dto.IndicatorCalculationResultDto{}, findStatisticsError
	}

	contractKCandleVos := alignmentDomain.Aligning(settlements, statistics)

	indicatorValues, executionError := contractIndicatorCalculationService.contractIndicatorScriptProxy.Execute(
		executionContext, requestDto.Script, calculationDomain.ResultType(), contractKCandleVos,
		calculationDomain.Parameters())
	if executionError != nil {
		return dto.IndicatorCalculationResultDto{}, executionError
	}

	// Bar open times in script order, so callers can align list values.
	openTimes := make([]time.Time, 0, len(contractKCandleVos))
	for _, contractKCandleVo := range contractKCandleVos {
		openTimes = append(openTimes, time.Unix(contractKCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	return calculationDomain.ToResultDto(openTimes, indicatorValues), nil
}

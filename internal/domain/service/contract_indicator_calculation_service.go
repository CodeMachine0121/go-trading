package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"golang.org/x/sync/errgroup"
)

// ContractIndicatorCalculationService is the application layer's only entry point for running indicator scripts over contract bars; request rules are the spot calculation's, with a round-the-clock calendar.
type ContractIndicatorCalculationService struct {
	summarizedSeriesReader                  kCandleContractSummarizedSeriesReader
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository
	contractPositionStatisticRepository     domaininterface.IContractPositionStatisticRepository
	contractIndicatorScriptProxy            domaininterface.IContractIndicatorScriptProxy
	contractOrderFlowService                *ContractOrderFlowService
	clockProxy                              domaininterface.IClockProxy
	roundTheClockMarket                     domains.MarketDomain
	maxCandleCount                          int
}

func NewContractIndicatorCalculationService(
	kCandleContractRepository domaininterface.IKCandleContractRepository,
	kCandleContractSummaryDayRepository domaininterface.IKCandleContractSummaryDayRepository,
	contractFundingRateSettlementRepository domaininterface.IContractFundingRateSettlementRepository,
	contractPositionStatisticRepository domaininterface.IContractPositionStatisticRepository,
	contractIndicatorScriptProxy domaininterface.IContractIndicatorScriptProxy,
	contractOrderFlowService *ContractOrderFlowService,
	clockProxy domaininterface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	maxCandleCount int,
) *ContractIndicatorCalculationService {
	return &ContractIndicatorCalculationService{
		summarizedSeriesReader: newKCandleContractSummarizedSeriesReader(
			kCandleContractRepository, kCandleContractSummaryDayRepository),
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		contractPositionStatisticRepository:     contractPositionStatisticRepository,
		contractIndicatorScriptProxy:            contractIndicatorScriptProxy,
		contractOrderFlowService:                contractOrderFlowService,
		clockProxy:                              clockProxy,
		roundTheClockMarket:                     marketCatalogDomain.MarketOf(string(vo.MarketCrypto)),
		maxCandleCount:                          maxCandleCount,
	}
}

// CalculateContractIndicator runs the script over the requested number of finished contract bars reading stored summaries where they exist and the funding and positioning it needs at once; nothing is fetched to fill gaps, except order flow, which is never stored.
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

	sourceKCandleContracts, findCandlesError := contractIndicatorCalculationService.summarizedSeriesReader.
		readLatestBuckets(executionContext, calculationDomain.SourceReach(),
			contractIndicatorCalculationService.clockProxy.Now())
	if findCandlesError != nil {
		return dto.IndicatorCalculationResultDto{}, findCandlesError
	}

	alignmentDomain, selectionError := calculationDomain.SelectContractInput(sourceKCandleContracts)
	if selectionError != nil {
		return dto.IndicatorCalculationResultDto{}, selectionError
	}

	settlements := make([]entities.ContractFundingRateSettlement, 0)
	leadInSettlement := entities.ContractFundingRateSettlement{}
	hasLeadIn := false
	statistics := make([]entities.ContractPositionStatistic, 0)
	reading, readingContext := errgroup.WithContext(executionContext)
	reading.Go(func() error {
		foundSettlements, findSettlementsError := contractIndicatorCalculationService.
			contractFundingRateSettlementRepository.
			FindInRange(readingContext, alignmentDomain.SettlementQuery(), alignmentDomain.SettlementReadLimit())
		settlements = foundSettlements

		return findSettlementsError
	})
	reading.Go(func() error {
		foundLeadIn, foundHasLeadIn, findLeadInError := contractIndicatorCalculationService.
			contractFundingRateSettlementRepository.
			FindLatestBefore(readingContext, calculationDomain.Symbol(), alignmentDomain.SettlementLeadInCutoff())
		leadInSettlement, hasLeadIn = foundLeadIn, foundHasLeadIn

		return findLeadInError
	})
	reading.Go(func() error {
		statisticTimes, findTimesError := contractIndicatorCalculationService.contractPositionStatisticRepository.
			FindStatisticTimesInRange(readingContext, alignmentDomain.StatisticQuery(), alignmentDomain.StatisticReadLimit())
		if findTimesError != nil {
			return findTimesError
		}

		foundStatistics, findStatisticsError := contractIndicatorCalculationService.contractPositionStatisticRepository.
			FindAtTimes(readingContext, calculationDomain.Symbol(), alignmentDomain.StatisticTimesInUse(statisticTimes))
		statistics = foundStatistics

		return findStatisticsError
	})
	if readingError := reading.Wait(); readingError != nil {
		return dto.IndicatorCalculationResultDto{}, readingError
	}
	if hasLeadIn {
		settlements = append(settlements, leadInSettlement)
	}

	contractKCandleVos, attachError := contractIndicatorCalculationService.contractOrderFlowService.AttachFootprints(
		executionContext, alignmentDomain, alignmentDomain.Aligning(settlements, statistics),
		calculationDomain.Parameters(), requestDto.OrderFlowPreparationAllowance)
	if attachError != nil {
		return dto.IndicatorCalculationResultDto{}, attachError
	}

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

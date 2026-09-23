package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractIndicatorCalculationService is the application layer's only entry point for
// running a user-written indicator script over perpetual contract bars.
//
// Every rule about what is asked — the observation window, the coarseness, how many
// buckets, which of them have finished, the knobs, the kind of value — is the spot
// calculation's, word for word, and lives in the same model. Perpetual contracts never
// close, so the one thing this takes from the market catalogue is the round-the-clock
// calendar. What is its own is the reading: contract K candles instead of spot ones,
// and the funding settlements and position statistics lined up beside them.
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

// CalculateContractIndicator runs the script over the requested number of perpetual
// contract bars for the trading symbol, taking only buckets that have finished, and
// reports one value per indicator name in the kind the request declared.
//
// It reads what is stored and nothing more: four reads — the contract K candles, then
// the funding settlements over the stretch those candles turned out to cover, the one
// settlement in force before it, and the position statistics — however many bars
// there are. Nothing is fetched from the venue
// to fill a gap; a stretch only partly stored is answered over what is there, exactly
// as a spot one is.
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

	// Where each bar the script saw begins, in the same order the script saw them, so
	// that a caller can put a list of values back where they belong.
	openTimes := make([]time.Time, 0, len(contractKCandleVos))
	for _, contractKCandleVo := range contractKCandleVos {
		openTimes = append(openTimes, time.Unix(contractKCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	return calculationDomain.ToResultDto(openTimes, indicatorValues), nil
}

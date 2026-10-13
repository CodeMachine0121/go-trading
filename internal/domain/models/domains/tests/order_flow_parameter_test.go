package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const orderFlowOnlyForContracts = "訂單流價位寬度只提供給合約行情種類的策略腳本"

func orderFlowPriceStepParameter(name string, defaultValue float64) dto.StrategyScriptParameterWriteDto {
	return dto.StrategyScriptParameterWriteDto{Name: name, Kind: "orderFlowPriceStep", DefaultValue: defaultValue}
}

func contractStrategyScriptWriteDto(parameters ...dto.StrategyScriptParameterWriteDto) dto.StrategyScriptWriteDto {
	writeDto := aStrategyScriptWriteDto()
	writeDto.MarketDataKind = "contractKCandle"
	writeDto.Parameters = parameters

	return writeDto
}

func TestAContractStrategyScriptDeclaresAnOrderFlowPriceStep(t *testing.T) {
	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(
		contractStrategyScriptWriteDto(orderFlowPriceStepParameter("價位寬度", 10)))

	require.NoError(t, validationError)
	storedParameters := strategyScriptDomain.ToEntity().Parameters
	require.Len(t, storedParameters, 1)
	assert.Equal(t, "orderFlowPriceStep", storedParameters[0].Kind)
	assert.Equal(t, 10.0, storedParameters[0].DefaultValue)
}

func TestAKCandleStrategyScriptCannotDeclareAnOrderFlowPriceStep(t *testing.T) {
	writeDto := aStrategyScriptWriteDto()
	writeDto.Parameters = []dto.StrategyScriptParameterWriteDto{orderFlowPriceStepParameter("價位寬度", 10)}

	_, validationError := domains.NewStrategyScriptDomain(writeDto)

	require.ErrorIs(t, validationError, domains.ErrStrategyScriptValidation)
	assert.Contains(t, validationError.Error(), orderFlowOnlyForContracts)
}

func TestOrderFlowPriceStepDeclarationRules(t *testing.T) {
	testCases := []struct {
		name             string
		declared         []dto.StrategyScriptParameterWriteDto
		expectedFragment string
	}{
		{
			name: "至多一個",
			declared: []dto.StrategyScriptParameterWriteDto{
				orderFlowPriceStepParameter("寬", 10), orderFlowPriceStepParameter("窄", 1)},
			expectedFragment: "訂單流價位寬度至多宣告一個",
		},
		{
			name:             "預設值為零",
			declared:         []dto.StrategyScriptParameterWriteDto{orderFlowPriceStepParameter("寬", 0)},
			expectedFragment: `參數 "寬" 是訂單流價位寬度，必須大於零`,
		},
		{
			name:             "預設值為負",
			declared:         []dto.StrategyScriptParameterWriteDto{orderFlowPriceStepParameter("寬", -1)},
			expectedFragment: `參數 "寬" 是訂單流價位寬度，必須大於零`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewStrategyScriptParametersDomain(testCase.declared)

			require.ErrorIs(t, validationError, domains.ErrStrategyScriptParameterValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedFragment)
		})
	}
}

func TestOrderFlowPriceStepMayBeAFraction(t *testing.T) {
	parameters := parametersOf(t, orderFlowPriceStepParameter("寬", 0.5))

	priceStep, declaresOrderFlow := parameters.OrderFlowPriceStep()

	require.True(t, declaresOrderFlow)
	assert.Equal(t, 0.5, priceStep.Value())
}

func TestThisRunsOrderFlowPriceStepMustBePositive(t *testing.T) {
	parameters := parametersOf(t, orderFlowPriceStepParameter("寬", 10))

	_, applyError := parameters.Applying([]dto.StrategyScriptParameterValueDto{{Name: "寬", Value: -5}})

	require.ErrorIs(t, applyError, domains.ErrStrategyScriptParameterValidation)
	assert.Contains(t, applyError.Error(), `參數 "寬" 是訂單流價位寬度，必須大於零`)
}

func TestThisRunsOrderFlowPriceStepReplacesTheDefault(t *testing.T) {
	parameters := parametersOf(t, orderFlowPriceStepParameter("寬", 10))

	applied, applyError := parameters.Applying([]dto.StrategyScriptParameterValueDto{{Name: "寬", Value: 25}})
	require.NoError(t, applyError)

	priceStep, declaresOrderFlow := applied.OrderFlowPriceStep()
	require.True(t, declaresOrderFlow)
	assert.Equal(t, 25.0, priceStep.Value())
}

func TestParametersWithoutAnOrderFlowPriceStepAskForNoOrderFlow(t *testing.T) {
	parameters := parametersOf(t, numberParameter("倍數", 3), lookbackCountParameter("期數", 20))

	_, declaresOrderFlow := parameters.OrderFlowPriceStep()

	assert.False(t, declaresOrderFlow)
}

func TestASpotCalculationRefusesAnOrderFlowDeclaration(t *testing.T) {
	requestDto := dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		StartTime:           momentAt("2026-09-03T06:00:00Z"),
		EndTime:             momentAt("2026-09-03T08:00:00Z"),
		AggregationInterval: "1h",
		Parameters:          []dto.StrategyScriptParameterWriteDto{orderFlowPriceStepParameter("寬", 10)},
	}
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, cryptoMarket(), maxCandleCount, calculationNow)
	require.NoError(t, validationError)

	spotError := calculationDomain.RequireSpotParameters()

	require.ErrorIs(t, spotError, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, spotError.Error(), orderFlowOnlyForContracts)
}

func TestASpotCalculationWithoutOrderFlowPasses(t *testing.T) {
	requestDto := dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		StartTime:           momentAt("2026-09-03T06:00:00Z"),
		EndTime:             momentAt("2026-09-03T08:00:00Z"),
		AggregationInterval: "1h",
		Parameters:          []dto.StrategyScriptParameterWriteDto{numberParameter("倍數", 3)},
	}
	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, cryptoMarket(), maxCandleCount, calculationNow)
	require.NoError(t, validationError)

	assert.NoError(t, calculationDomain.RequireSpotParameters())
}

func TestASpotReplayRefusesAnOrderFlowDeclaration(t *testing.T) {
	requestDto := backtestRequest()
	requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{orderFlowPriceStepParameter("寬", 10)}
	backtestDomain, validationError := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)
	require.NoError(t, validationError)

	spotError := backtestDomain.RequireSpotParameters()

	require.ErrorIs(t, spotError, domains.ErrBacktestValidation)
	assert.Contains(t, spotError.Error(), orderFlowOnlyForContracts)
}

func TestASpotTradingStrategyReplayRefusesASourceDeclaringOrderFlow(t *testing.T) {
	requestDto := tradingStrategyBacktestRequest()
	requestDto.SignalSources[0].Parameters = []dto.StrategyScriptParameterWriteDto{
		orderFlowPriceStepParameter("寬", 10)}

	_, validationError := domains.NewTradingStrategyBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)

	require.ErrorIs(t, validationError, domains.ErrBacktestValidation)
	assert.Contains(t, validationError.Error(), orderFlowOnlyForContracts)
}

func TestOrderFlowPriceLevels(t *testing.T) {
	testCases := []struct {
		name          string
		priceStep     float64
		price         string
		expectedPrice float64
	}{
		{name: "往下取到寬度的整數倍", priceStep: 10, price: "84157", expectedPrice: 84150},
		{name: "正好在邊界上屬於那一個價位", priceStep: 10, price: "84160", expectedPrice: 84160},
		{name: "小數寬度的邊界不被浮點誤差推到下一格", priceStep: 0.1, price: "0.3", expectedPrice: 0.3},
		{name: "邊界下一點點屬於下面那一格", priceStep: 0.1, price: "0.2999", expectedPrice: 0.2},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			priceStep, stepError := domains.NewOrderFlowPriceStepDomain(testCase.priceStep)
			require.NoError(t, stepError)

			level := priceStep.LevelOf(decimal.RequireFromString(testCase.price))

			assert.Equal(t, testCase.expectedPrice, priceStep.PriceOf(level))
		})
	}
}

func TestOrderFlowPriceStepsWithTheSameWidthShareAKey(t *testing.T) {
	ten, _ := domains.NewOrderFlowPriceStepDomain(10)
	alsoTen, _ := domains.NewOrderFlowPriceStepDomain(10.0)
	twentyFive, _ := domains.NewOrderFlowPriceStepDomain(25)

	assert.Equal(t, ten.Key(), alsoTen.Key())
	assert.NotEqual(t, ten.Key(), twentyFive.Key())
}

func TestOrderFlowPriceStepIsNoFinerThanEightDecimals(t *testing.T) {
	_, finestError := domains.NewOrderFlowPriceStepDomain(0.00000001)
	_, tooFineError := domains.NewOrderFlowPriceStepDomain(0.000000009)

	assert.NoError(t, finestError)
	require.Error(t, tooFineError)
	assert.Contains(t, tooFineError.Error(), "是訂單流價位寬度，至少 0.00000001")
}

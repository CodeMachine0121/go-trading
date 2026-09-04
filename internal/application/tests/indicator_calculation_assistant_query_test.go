package application_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type indicatorCalculationAssistantQueryUnderTest struct {
	assistantQuery       *application.IndicatorCalculationAssistantQuery
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
	strategyRepository   *mocks.MockIStrategyRepository
}

// newIndicatorCalculationAssistantQueryUnderTest wires the real domain services and
// real domain models, mocking only storage, the clock and script execution.
func newIndicatorCalculationAssistantQueryUnderTest(t *testing.T) indicatorCalculationAssistantQueryUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(indicatorNow).AnyTimes()

	return indicatorCalculationAssistantQueryUnderTest{
		assistantQuery: application.NewIndicatorCalculationAssistantQuery(
			application.NewIndicatorCalculationApplication(
				service.NewIndicatorCalculationService(
					kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults)),
			application.NewStrategyApplication(service.NewStrategyService(strategyRepository)),
		),
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
		strategyRepository:   strategyRepository,
	}
}

// expectMarketRead answers with this many candles, newest first, which is the order a
// read as of a moment comes back in. It is generous by default because a strategy that
// declares a lookback needs that many more candles than the count asked for.
func (fixture indicatorCalculationAssistantQueryUnderTest) expectMarketRead(candleCount int) {
	kCandles := make([]entities.KCandle, 0, candleCount)
	for candleNumber := range candleCount {
		kCandles = append(kCandles, kCandleAt(at(9, 10-candleNumber*5), "100"))
	}

	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(kCandles, nil)
}

func TestIndicatorCalculationAssistantQueryRunsAnAlgorithmTheAssistantBrought(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.expectMarketRead(40)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "func Calculate() {}", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":2,"script":"func Calculate() {}"}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"ma":110`)
	assert.Contains(t, outcome, `"usedCandleCount":2`)
}

func TestIndicatorCalculationAssistantQueryRunsTheStrategyItNames(t *testing.T) {
	// Naming a strategy is how the question is actually asked. Making the assistant
	// read it and send the algorithm back would cost a round trip and put the whole
	// script through the conversation twice for nothing.
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil)
	fixture.expectMarketRead(40)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), aStoredStrategyWithKnobs(1, "二十根均線").Script, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":2,"strategyId":1,`+
			`"parameterValues":[{"name":"lookback","value":30}]}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"ma"`)
	// The value kind that runs is the strategy's own, whatever arrived alongside it.
	assert.Contains(t, outcome, `"resultType":"floatList"`)
}

func TestIndicatorCalculationAssistantQueryPrefersTheNamedStrategyOverAnAlgorithmSentWithIt(t *testing.T) {
	// The two must never quietly disagree about which one ran.
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil)
	fixture.expectMarketRead(40)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), aStoredStrategyWithKnobs(1, "二十根均線").Script, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	_, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":2,"strategyId":1,"script":"func Other() {}"}`)

	require.NoError(t, runError)
}

func TestIndicatorCalculationAssistantQueryReadsUpToTheMomentItWasGiven(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", at(9, 0), gomock.Any()).
		Return([]entities.KCandle{
			kCandleAt(at(8, 55), "100"), kCandleAt(at(8, 50), "100"), kCandleAt(at(8, 45), "100"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	_, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":2,"script":"func Calculate() {}",`+
			`"endTime":"2026-08-29T09:00:00Z"}`)

	require.NoError(t, runError)
}

func TestIndicatorCalculationAssistantQueryReportsAStrategyThatIsNotThere(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Strategy{}, domains.StrategyNotFound(99))

	_, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":2,"strategyId":99}`)

	require.ErrorIs(t, runError, domains.ErrStrategyNotFound)
}

func TestIndicatorCalculationAssistantQueryIsBoundByTheRulesTheCalculationAlreadyHas(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

	_, runError := fixture.assistantQuery.Run(t.Context(),
		`{"symbol":"BTCUSDT","candleCount":0,"script":"func Calculate() {}"}`)

	require.ErrorIs(t, runError, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, runError.Error(), "計算根數必須大於零")
}

func TestIndicatorCalculationAssistantQueryRefusesArgumentsItCannotRead(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       string
		expectedMessage string
	}{
		{name: "not JSON at all", arguments: `nope`, expectedMessage: "不是合法的 JSON"},
		{
			name:            "a moment that is not a moment",
			arguments:       `{"symbol":"BTCUSDT","candleCount":2,"endTime":"昨天"}`,
			expectedMessage: "RFC3339",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

			_, runError := fixture.assistantQuery.Run(t.Context(), testCase.arguments)

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

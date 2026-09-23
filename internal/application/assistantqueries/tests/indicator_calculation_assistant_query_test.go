package assistantqueries_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
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
	assistantQuery           *assistantqueries.IndicatorCalculationAssistantQuery
	kCandleRepository        *mocks.MockIKCandleRepository
	indicatorScriptProxy     *mocks.MockIIndicatorScriptProxy
	strategyScriptRepository *mocks.MockIStrategyScriptRepository
}

// newIndicatorCalculationAssistantQueryUnderTest wires the real domain services and
// real domain models, mocking only storage, the clock and script execution.
func newIndicatorCalculationAssistantQueryUnderTest(t *testing.T) indicatorCalculationAssistantQueryUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(indicatorNow).AnyTimes()

	return indicatorCalculationAssistantQueryUnderTest{
		assistantQuery: assistantqueries.NewIndicatorCalculationAssistantQuery(
			application.NewIndicatorCalculationApplication(
				service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
				service.NewIndicatorCalculationService(
					kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
					domains.NewMarketCatalogDomain(
						map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
					queryMaxResults),
				// Nothing here calculates over contract bars.
				nil),
		),
		kCandleRepository:        kCandleRepository,
		indicatorScriptProxy:     indicatorScriptProxy,
		strategyScriptRepository: strategyScriptRepository,
	}
}

// expectMarketRead answers with this many candles, newest first, which is the order a
// read as of a moment comes back in. It is generous by default because a strategy script that
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

	outcome, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","script":"func Calculate() {}"}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"ma":110`)
	assert.Contains(t, outcome, `"usedCandleCount":2`)
}

func TestIndicatorCalculationAssistantQueryRunsTheStrategyScriptItNames(t *testing.T) {
	// Naming a strategy script is how the question is actually asked. Making the assistant
	// read it and send the algorithm back would cost a round trip and put the whole
	// script through the conversation twice for nothing.
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)
	fixture.expectMarketRead(40)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), aStoredStrategyScriptWithKnobs(1, "二十根均線").Script, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	outcome, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","strategyScriptId":1,`+
			`"parameterValues":[{"name":"lookback","value":30}]}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"ma"`)
	// The value kind that runs is the strategy script's own, whatever arrived alongside it.
	assert.Contains(t, outcome, `"resultType":"floatList"`)
}

func TestIndicatorCalculationAssistantQueryRefusesNamingAStrategyScriptAndSendingAnAlgorithm(t *testing.T) {
	// The two used to have a winner: the named strategy script quietly beat the algorithm
	// sent with it. Picking a winner is a decision nobody asked for, and the loser
	// disappears without a word — so both together is now refused outright, the
	// same way the K candle series refuses two ways of saying how coarse to look.
	//
	// Nothing is stubbed: the refusal lands before anything is read.
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","strategyScriptId":1,"script":"func Other() {}"}`)

	require.ErrorIs(t, runError, domains.ErrRunSubjectAmbiguous)
}

func TestIndicatorCalculationAssistantQueryRefusesNeitherAStrategyScriptNorAnAlgorithm(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z"}`)

	require.ErrorIs(t, runError, domains.ErrRunSubjectAmbiguous)
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

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T08:58:00Z","script":"func Calculate() {}",`+
			`"endTime":"2026-08-29T09:00:00Z"}`)

	require.NoError(t, runError)
}

func TestIndicatorCalculationAssistantQueryReportsAStrategyScriptThatIsNotThere(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(99))

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","strategyScriptId":99}`)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestIndicatorCalculationAssistantQueryIsBoundByTheRulesTheCalculationAlreadyHas(t *testing.T) {
	fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

	_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID,
		`{"symbol":"BTCUSDT","startTime":"2026-08-29T09:15:00Z","script":"func Calculate() {}"}`)

	require.ErrorIs(t, runError, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, runError.Error(), "起點必須早於終點")
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
			arguments:       `{"symbol":"BTCUSDT","startTime":"2026-08-29T09:13:00Z","endTime":"昨天"}`,
			expectedMessage: "RFC3339",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newIndicatorCalculationAssistantQueryUnderTest(t)

			_, runError := fixture.assistantQuery.Run(t.Context(), assistantViewerID, testCase.arguments)

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

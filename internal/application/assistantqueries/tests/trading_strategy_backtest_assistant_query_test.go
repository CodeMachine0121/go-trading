package assistantqueries_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// replayStart is where every stretch replayed below begins, and replayNow sits well
// after all of them so nothing is refused for reaching into an unfinished interval.
var (
	replayStart = time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	replayNow   = time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
)

type tradingStrategyBacktestAssistantQueryUnderTest struct {
	backtestAssistantQuery    *assistantqueries.TradingStrategyBacktestAssistantQuery
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	kCandleRepository         *mocks.MockIKCandleRepository
	indicatorScriptProxy      *mocks.MockIIndicatorScriptProxy
}

// newTradingStrategyBacktestAssistantQueryUnderTest wires the real application, the
// real domain services and the real models, mocking only storage and script
// execution — so the assistant replaying a set of rules runs the very same replay a
// person does.
func newTradingStrategyBacktestAssistantQueryUnderTest(
	t *testing.T,
) tradingStrategyBacktestAssistantQueryUnderTest {
	controller := gomock.NewController(t)

	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(replayNow).AnyTimes()

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: assistantViewerID, Script: "the script",
			}, nil
		}).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	tradingStrategyBacktestApplication := application.NewTradingStrategyBacktestApplication(
		service.NewTradingStrategyService(tradingStrategyRepository),
		service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
		service.NewBacktestService(
			kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults))

	return tradingStrategyBacktestAssistantQueryUnderTest{
		backtestAssistantQuery: assistantqueries.NewTradingStrategyBacktestAssistantQuery(
			tradingStrategyBacktestApplication),
		tradingStrategyRepository: tradingStrategyRepository,
		kCandleRepository:         kCandleRepository,
		indicatorScriptProxy:      indicatorScriptProxy,
	}
}

// aReplayableTradingStrategy is one source A on the given coarsenesses, buying on A's
// buy and selling on A's sell. Two coarsenesses means two sources, which is how a
// case asks for the mismatch refusal.
func aReplayableTradingStrategy(intervals ...string) entities.TradingStrategy {
	sources := make([]entities.TradingStrategySignalSource, 0, len(intervals))
	for index, interval := range intervals {
		sources = append(sources, entities.TradingStrategySignalSource{
			ID: uint(20 + index), TradingStrategyID: assistantTradingStrategyID,
			Label: string(rune('A' + index)), StrategyScriptID: uint(9 + index),
			AggregationInterval: interval,
		})
	}

	return entities.TradingStrategy{
		ID: assistantTradingStrategyID, OwnerID: assistantViewerID, Name: "動能追蹤",
		SignalSources: sources,
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 30, TradingStrategyID: assistantTradingStrategyID,
				Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 31, TradingStrategyID: assistantTradingStrategyID,
				Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

// replayedCandle builds a stored candle that many hours into the stretch.
func replayedCandle(hour int, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: replayStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.RequireFromString(closePrice),
		High:     decimal.RequireFromString(closePrice),
		Low:      decimal.RequireFromString(closePrice),
		Close:    decimal.RequireFromString(closePrice),
	}
}

// replaySignals is what one source's script said, one signal per candle.
func replaySignals(signals ...vo.SignalVo) []map[string]vo.IndicatorValueVo {
	perCandleIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(signals))
	for _, signal := range signals {
		perCandleIndicatorValues = append(perCandleIndicatorValues,
			map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: signal}})
	}

	return perCandleIndicatorValues
}

// aReplayArgument is what the assistant sends to replay the trading strategy.
const aReplayArgument = `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10000",
  "positionSizingMode": "allIn"
}`

// replayReport is the report as the assistant reads it, named here so a case can say
// which fields it expects to find and which it expects to be absent.
type replayReport struct {
	Symbol   string `json:"symbol"`
	Interval string `json:"interval"`
	Summary  struct {
		InitialCapital        string   `json:"initialCapital"`
		FinalEquity           string   `json:"finalEquity"`
		TotalReturnRate       float64  `json:"totalReturnRate"`
		MaximumDrawdown       float64  `json:"maximumDrawdown"`
		WinRate               *float64 `json:"winRate"`
		PositionOpenCount     int      `json:"positionOpenCount"`
		ConflictedCandleCount int      `json:"conflictedCandleCount"`
	} `json:"summary"`
	ClosedTrades []struct {
		Direction string `json:"direction"`
		Profit    string `json:"profit"`
	} `json:"closedTrades"`
}

func (fixture tradingStrategyBacktestAssistantQueryUnderTest) replay(
	t *testing.T, arguments string,
) (replayReport, string) {
	t.Helper()

	outcome, runError := fixture.backtestAssistantQuery.Run(
		t.Context(), assistantViewerID, arguments)
	require.NoError(t, runError)

	report := replayReport{}
	require.NoError(t, json.Unmarshal([]byte(outcome), &report))

	return report, outcome
}

func TestTradingStrategyBacktestAssistantQueryHandsBackTheReportCard(t *testing.T) {
	// The assistant writes an algorithm and then goes blind. This is the capability
	// that lets it see what it wrote actually did.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			replayedCandle(0, "100"), replayedCandle(1, "110"), replayedCandle(2, "120"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(replaySignals(vo.SignalBuy, vo.SignalHold, vo.SignalSell), nil)

	report, _ := fixture.replay(t, aReplayArgument)

	assert.Equal(t, "BTCUSDT", report.Symbol)
	assert.Equal(t, "1h", report.Interval)
	assert.Equal(t, "10000", report.Summary.InitialCapital)
	// Two openings for one round trip: the sell closes the long and turns around
	// into a short, which is still open when the replay ends.
	assert.Equal(t, 2, report.Summary.PositionOpenCount)
	require.Len(t, report.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionLong), report.ClosedTrades[0].Direction)
}

func TestTradingStrategyBacktestAssistantQueryLeavesTheEquityCurveOut(t *testing.T) {
	// A three-hundred-point curve rendered as text is three hundred numbers that
	// crowd out the report card the assistant actually has to read — and not one of
	// them is new: the trades plus the opening capital give it back.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			replayedCandle(0, "100"), replayedCandle(1, "110"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(replaySignals(vo.SignalHold, vo.SignalHold), nil)

	_, outcome := fixture.replay(t, aReplayArgument)

	assert.NotContains(t, outcome, "equityCurve")
}

func TestTradingStrategyBacktestAssistantQueryAlwaysReportsTheConflictCount(t *testing.T) {
	// Without it the assistant reads a report card with almost no trades as a very
	// steady strategy, when what happened is that the rules decided nothing at all.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aConflictingTradingStrategy(), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			replayedCandle(0, "100"), replayedCandle(1, "110"), replayedCandle(2, "120"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(replaySignals(vo.SignalBuy, vo.SignalBuy, vo.SignalBuy), nil)

	report, outcome := fixture.replay(t, aReplayArgument)

	assert.Contains(t, outcome, "conflictedCandleCount")
	assert.Equal(t, 3, report.Summary.ConflictedCandleCount)
	assert.Equal(t, 0, report.Summary.PositionOpenCount)
}

func TestTradingStrategyBacktestAssistantQueryHandsBackTheCoarsenessRefusal(t *testing.T) {
	// The refusal names the coarsenesses in play, and the assistant is the party
	// that can go and make them agree.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h", "5m"), nil)

	_, runError := fixture.backtestAssistantQuery.Run(
		t.Context(), assistantViewerID, aReplayArgument)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "1h")
	assert.Contains(t, runError.Error(), "5m")
}

func TestTradingStrategyBacktestAssistantQueryAnswersSomebodyElsesAsNotFound(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	strangersTradingStrategy := aReplayableTradingStrategy("1h")
	strangersTradingStrategy.OwnerID = assistantViewerID + 1
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(strangersTradingStrategy, nil)

	_, runError := fixture.backtestAssistantQuery.Run(
		t.Context(), assistantViewerID, aReplayArgument)

	require.ErrorIs(t, runError, domains.ErrTradingStrategyNotFound)
}

func TestTradingStrategyBacktestAssistantQueryRefusesArgumentsThatAreNotJson(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	_, runError := fixture.backtestAssistantQuery.Run(t.Context(), assistantViewerID, "not json")

	require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
}

func TestTradingStrategyBacktestAssistantQueryIsNamedForWhatItReplays(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	assert.Equal(t, "run_trading_strategy_backtest", fixture.backtestAssistantQuery.Name())
}

// aConflictingTradingStrategy buys and sells on the very same word, so every candle
// its source speaks on has both conditions holding at once.
func aConflictingTradingStrategy() entities.TradingStrategy {
	tradingStrategy := aReplayableTradingStrategy("1h")
	tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 30, TradingStrategyID: assistantTradingStrategyID,
			Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
		{ID: 31, TradingStrategyID: assistantTradingStrategyID,
			Side: "sell", SourceLabel: "A", ExpectedSignal: "buy"},
	}

	return tradingStrategy
}

func TestTradingStrategyBacktestAssistantQueryCapsTheTradesItHandsOverAndSaysSo(t *testing.T) {
	// What a capability hands back is replayed to the assistant on every later round
	// of the same answer. Four or five replays of a chatty strategy would carry every
	// trade of every attempt, and it would run out of room to think before it ran out
	// of queries.
	//
	// Being told is the other half: fifty trades out of six hundred, read as all of
	// them, describe a strategy that does not exist — and nothing about the list
	// itself gives that away.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h"), nil)

	// Alternating buy and sell on every candle is the shape that produces the most
	// round trips per candle there is.
	candleCount := 260
	candles := make([]entities.KCandle, 0, candleCount)
	signals := make([]vo.SignalVo, 0, candleCount)
	for candleIndex := range candleCount {
		candles = append(candles, replayedCandle(candleIndex, "100"))
		if candleIndex%2 == 0 {
			signals = append(signals, vo.SignalBuy)
			continue
		}
		signals = append(signals, vo.SignalSell)
	}

	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(candles, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(replaySignals(signals...), nil)

	report, outcome := fixture.replay(t, `{
      "tradingStrategyId": 11,
      "symbol": "BTCUSDT",
      "startTime": "2026-09-10T00:00:00Z",
      "endTime": "2026-09-20T00:00:00Z",
      "initialCapital": "10000",
      "positionSizingMode": "allIn"
    }`)

	assert.Len(t, report.ClosedTrades, 50)
	assert.Contains(t, outcome, "只列出最近的 50 筆")
	// The report card still counts every one of them — that is why leaving trades out
	// costs nothing that matters.
	assert.Greater(t, report.Summary.PositionOpenCount, 50)
}

func TestTradingStrategyBacktestAssistantQuerySaysNothingAboutTruncationWhenNothingWasLeftOut(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			replayedCandle(0, "100"), replayedCandle(1, "110"), replayedCandle(2, "120"),
		}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(replaySignals(vo.SignalBuy, vo.SignalHold, vo.SignalSell), nil)

	_, outcome := fixture.replay(t, aReplayArgument)

	assert.NotContains(t, outcome, "closedTradesTruncated")
}

// aSpotReplayArgument replays the same stretch the way an account that cannot short
// would actually have traded it.
const aSpotReplayArgument = `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10000",
  "positionSizingMode": "allIn",
  "tradingMode": "spot"
}`

// The assistant converges by replaying, reading and adjusting. Handing it only the
// mode that can short would have it tune a set of rules against trades the person's
// account can never place — and nothing in the report card would say so.
func TestTradingStrategyBacktestAssistantQueryReplaysTheWayTheAccountTrades(t *testing.T) {
	t.Run("naming the long only mode keeps every round trip long", func(t *testing.T) {
		fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().
			FindOne(gomock.Any(), assistantTradingStrategyID).
			Return(aReplayableTradingStrategy("1h"), nil)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				replayedCandle(0, "100"), replayedCandle(1, "120"), replayedCandle(2, "90"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(replaySignals(vo.SignalBuy, vo.SignalSell, vo.SignalHold), nil)

		report, _ := fixture.replay(t, aSpotReplayArgument)

		// Sold at 120 and stood aside for the fall to 90.
		assert.Equal(t, "12000", report.Summary.FinalEquity)
		assert.Equal(t, 1, report.Summary.PositionOpenCount)
		require.Len(t, report.ClosedTrades, 1)
		assert.Equal(t, string(vo.PositionDirectionLong), report.ClosedTrades[0].Direction)
	})

	t.Run("naming no mode replays the way it always has", func(t *testing.T) {
		fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().
			FindOne(gomock.Any(), assistantTradingStrategyID).
			Return(aReplayableTradingStrategy("1h"), nil)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				replayedCandle(0, "100"), replayedCandle(1, "120"), replayedCandle(2, "90"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(replaySignals(vo.SignalBuy, vo.SignalSell, vo.SignalHold), nil)

		report, _ := fixture.replay(t, aReplayArgument)

		// The 12,000 went straight back out as a short at 120, and the fall paid it.
		assert.Equal(t, "15000", report.Summary.FinalEquity)
		assert.Equal(t, 2, report.Summary.PositionOpenCount)
	})

	t.Run("a mode nobody offers is refused rather than replayed", func(t *testing.T) {
		fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
		fixture.tradingStrategyRepository.EXPECT().
			FindOne(gomock.Any(), assistantTradingStrategyID).
			Return(aReplayableTradingStrategy("1h"), nil)

		outcome, runError := fixture.backtestAssistantQuery.Run(
			t.Context(), assistantViewerID, `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10000",
  "positionSizingMode": "allIn",
  "tradingMode": "dayTrade"
}`)

		require.Error(t, runError)
		assert.ErrorIs(t, runError, domains.ErrBacktestValidation)
		// Nothing partial reaches the assistant: half a report card would be read as
		// a whole one.
		assert.Empty(t, outcome)
	})
}

// The assistant can only pick the right mode if it is told what the two mean and
// which one it gets by saying nothing.
func TestTradingStrategyBacktestAssistantQueryExplainsTheTwoWaysToTrade(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	argumentSchema := fixture.backtestAssistantQuery.ArgumentSchema()
	description := fixture.backtestAssistantQuery.Description()

	assert.Contains(t, argumentSchema, "tradingMode")
	assert.Contains(t, argumentSchema, string(vo.TradingModeLongShort))
	assert.Contains(t, argumentSchema, string(vo.TradingModeSpot))
	// Not required: saying nothing has to stay a legitimate thing to do, or every
	// existing answer would start failing.
	requiredArguments := struct {
		Required []string `json:"required"`
	}{}
	require.NoError(t, json.Unmarshal([]byte(argumentSchema), &requiredArguments))
	assert.NotContains(t, requiredArguments.Required, "tradingMode")

	assert.Contains(t, description, string(vo.TradingModeLongShort))
	assert.Contains(t, description, string(vo.TradingModeSpot))
}

package assistantqueries_test

import (
	"context"
	"encoding/json"
	"strings"
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
			kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, time.Minute), nil)

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
		TotalTransactionCost  string   `json:"totalTransactionCost"`
		StopLossExitCount     int      `json:"stopLossExitCount"`
	} `json:"summary"`
	ClosedTrades []struct {
		Direction  string `json:"direction"`
		Profit     string `json:"profit"`
		EntryCost  string `json:"entryCost"`
		ExitCost   string `json:"exitCost"`
		ExitReason string `json:"exitReason"`
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
	// One opening and one round trip: the sell closed back to cash rather than
	// turning around.
	assert.Equal(t, 1, report.Summary.PositionOpenCount)
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

// The assistant converges by replaying, reading and adjusting. There is one set of
// rules to replay by, so there is nothing for it to declare and nothing to get wrong.
func TestTradingStrategyBacktestAssistantQueryReplaysSpot(t *testing.T) {
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

	// Sold at 120 and stood aside for the fall to 90.
	assert.Equal(t, "12000", report.Summary.FinalEquity)
	assert.Equal(t, 1, report.Summary.PositionOpenCount)
	require.Len(t, report.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionLong), report.ClosedTrades[0].Direction)
}

// What a broker charges is the assistant's to say: a set of rules has no opinion about
// it, and the person asking "does this still make money after fees" is asking a
// question only this capability can answer.
//
// The two exit distances are here for the same reason, and they arrived late: the
// capability was built without them, which left the assistant handing back a report
// card the person could not reproduce from the screen. Three doors into one replay
// have to offer the same boxes, or the difference between two report cards says
// nothing about the strategy.
func TestTradingStrategyBacktestAssistantQueryChargesWhatTheAssistantSaysItCosts(t *testing.T) {
	replayingWith := func(t *testing.T, costArguments string) replayReport {
		t.Helper()

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

		report, _ := fixture.replay(t, `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10100",
  "positionSizingMode": "allIn"`+costArguments+`
}`)

		return report
	}

	t.Run("naming rates charges them", func(t *testing.T) {
		report := replayingWith(t, `,
  "entryCostPercentage": "1",
  "exitCostPercentage": "1"`)

		assert.Equal(t, "11880", report.Summary.FinalEquity)
		assert.Equal(t, "220", report.Summary.TotalTransactionCost)
		require.Len(t, report.ClosedTrades, 1)
		assert.Equal(t, "100", report.ClosedTrades[0].EntryCost)
		assert.Equal(t, "120", report.ClosedTrades[0].ExitCost)
		assert.Equal(t, "1780", report.ClosedTrades[0].Profit)
	})

	t.Run("naming none leaves the report card exactly as it was", func(t *testing.T) {
		report := replayingWith(t, "")

		assert.Equal(t, "12120", report.Summary.FinalEquity)
		assert.Equal(t, "0", report.Summary.TotalTransactionCost)
		require.Len(t, report.ClosedTrades, 1)
		assert.Equal(t, "2020", report.ClosedTrades[0].Profit)
	})

	t.Run("a rate nobody could charge is refused, and nothing partial gets through", func(t *testing.T) {
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
  "initialCapital": "10100",
  "positionSizingMode": "allIn",
  "entryCostPercentage": "-1"
}`)

		require.Error(t, runError)
		assert.ErrorIs(t, runError, domains.ErrBacktestValidation)
		assert.Empty(t, outcome)
	})
}

// Declaring a box is not the same as honouring it. The two exit distances arrived on
// this capability late, and a schema that offers them while the value goes nowhere
// would pass every assertion about the schema and still hand back a report card with
// no stop in it.
func TestTradingStrategyBacktestAssistantQuerySimulatesTheExitDistancesItWasGiven(t *testing.T) {
	replayingWith := func(t *testing.T, exitArguments string) replayReport {
		t.Helper()

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
			Return(replaySignals(vo.SignalBuy, vo.SignalHold, vo.SignalHold), nil)

		report, _ := fixture.replay(t, `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10100",
  "positionSizingMode": "allIn"`+exitArguments+`
}`)

		return report
	}

	t.Run("naming a stop gets the bet taken off at it", func(t *testing.T) {
		report := replayingWith(t, `,
  "stopLossPercentage": "2"`)

		require.Len(t, report.ClosedTrades, 1)
		assert.Equal(t, string(vo.TradeExitReasonStopLoss), report.ClosedTrades[0].ExitReason)
		assert.Equal(t, 1, report.Summary.StopLossExitCount)
		// 101 units bought at 100, taken off at 98.
		assert.Equal(t, "9898", report.Summary.FinalEquity)
	})

	t.Run("naming none rides the fall all the way down", func(t *testing.T) {
		report := replayingWith(t, "")

		assert.Empty(t, report.ClosedTrades)
		assert.Equal(t, 0, report.Summary.StopLossExitCount)
		assert.Equal(t, "9090", report.Summary.FinalEquity)
	})
}

// Four boxes the assistant may fill and none it must. A required cost rate would make
// every replay an argument about fees; an absent one would make the assistant guess.
func TestTradingStrategyBacktestAssistantQueryOffersCostsAndExitsWithoutDemandingThem(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	argumentSchema := fixture.backtestAssistantQuery.ArgumentSchema()

	// The schema is assembled by hand out of string pieces, so the first thing worth
	// asserting is that it is still a document at all. A broken one does not fail
	// here — it fails wherever the assistant is told about its tools, far from the
	// edit that broke it.
	require.True(t, json.Valid([]byte(argumentSchema)), "argument schema is not valid JSON")

	requiredArguments := argumentSchema[strings.Index(argumentSchema, `"required":`):]
	for _, optionalArgument := range []string{
		"entryCostPercentage", "exitCostPercentage",
		"stopLossPercentage", "takeProfitPercentage",
	} {
		assert.Contains(t, argumentSchema, optionalArgument)
		assert.NotContains(t, requiredArguments, optionalArgument)
	}

	// It also has to know that leaving them out is not neutral — a report card with no
	// fees in it is the one that flatters a strategy that trades constantly.
	assert.Contains(t, fixture.backtestAssistantQuery.Description(), "totalTransactionCost")
	assert.Contains(t, fixture.backtestAssistantQuery.Description(), "淨額")
}

// The assistant has no set of rules to name and no loan to ask for, and the tool says
// so — otherwise it would keep offering somebody a choice that is refused on arrival.
func TestTradingStrategyBacktestAssistantQueryOffersNoModeAndNoBorrowing(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	argumentSchema := fixture.backtestAssistantQuery.ArgumentSchema()
	description := fixture.backtestAssistantQuery.Description()

	// No box for either, and nothing else may be sent.
	assert.NotContains(t, argumentSchema, "tradingMode")
	assert.NotContains(t, argumentSchema, "leverage")
	assert.NotContains(t, argumentSchema, "maintenanceMarginRate")
	assert.Contains(t, argumentSchema, `"additionalProperties":false`)

	// And it is told what this replay does, so that somebody asking to short or to
	// borrow gets an answer rather than a setting substituted on their behalf.
	assert.Contains(t, description, "只做現貨")
	assert.Contains(t, description, "沒有交易模式可以給")
	assert.NotContains(t, description, "liquidationExitCount")
}

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

// replayNow sits well after every stretch starting at replayStart, so none reaches into an unfinished interval.
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

// newTradingStrategyBacktestAssistantQueryUnderTest mocks only storage and script execution, so the assistant runs the same replay a person does.
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

// aReplayableTradingStrategy has one source A per interval, buying on A's buy and selling on A's sell; two intervals trigger the mismatch refusal.
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

const aReplayArgument = `{
  "tradingStrategyId": 11,
  "symbol": "BTCUSDT",
  "startTime": "2026-09-10T00:00:00Z",
  "endTime": "2026-09-10T04:00:00Z",
  "initialCapital": "10000",
  "positionSizingMode": "allIn"
}`

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
	// One opening: the sell closed to cash rather than reversing.
	assert.Equal(t, 1, report.Summary.PositionOpenCount)
	require.Len(t, report.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionLong), report.ClosedTrades[0].Direction)
}

func TestTradingStrategyBacktestAssistantQueryLeavesTheEquityCurveOut(t *testing.T) {
	// The equity curve is omitted since it crowds the report and is derivable from the trades and opening capital.
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
	// Without this, a report with almost no trades reads as a steady strategy rather than one that decided nothing.
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
	// The refusal names the coarsenesses so the assistant can make them agree.
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

// aConflictingTradingStrategy buys and sells on the same signal, so both conditions hold on every candle its source speaks on.
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
	// Trades are capped because the result is replayed on every later round of the answer, and the cap is reported so a partial list isn't read as complete.
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aReplayableTradingStrategy("1h"), nil)

	// Alternating buy and sell every candle maximises round trips.
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
	// The summary still counts every trade, so omitting some loses nothing that matters.
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

// Spot is the only mode, so there is nothing for the assistant to declare.
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

// Transaction costs and the two exit distances are the assistant's to set, matching the other replay entry points so report cards stay comparable.
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

// Checks the exit distances are actually simulated, not merely declared in the schema.
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
		// 101 units bought at 100; the flat bar at 90 gaps through the 98 stop, so they come off at 90.
		assert.Equal(t, "9090", report.Summary.FinalEquity)
	})

	t.Run("naming none rides the fall all the way down", func(t *testing.T) {
		report := replayingWith(t, "")

		assert.Empty(t, report.ClosedTrades)
		assert.Equal(t, 0, report.Summary.StopLossExitCount)
		assert.Equal(t, "9090", report.Summary.FinalEquity)
	})
}

// All four fields are optional: a required cost rate would force a fee argument on every replay.
func TestTradingStrategyBacktestAssistantQueryOffersCostsAndExitsWithoutDemandingThem(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	argumentSchema := fixture.backtestAssistantQuery.ArgumentSchema()

	// The schema is hand-assembled from strings, so check it is valid JSON here rather than failing far away when tools are described.
	require.True(t, json.Valid([]byte(argumentSchema)), "argument schema is not valid JSON")

	requiredArguments := argumentSchema[strings.Index(argumentSchema, `"required":`):]
	for _, optionalArgument := range []string{
		"entryCostPercentage", "exitCostPercentage",
		"stopLossPercentage", "takeProfitPercentage",
	} {
		assert.Contains(t, argumentSchema, optionalArgument)
		assert.NotContains(t, requiredArguments, optionalArgument)
	}

	// The description warns that omitting costs flatters strategies that trade constantly.
	assert.Contains(t, fixture.backtestAssistantQuery.Description(), "totalTransactionCost")
	assert.Contains(t, fixture.backtestAssistantQuery.Description(), "淨額")
}

// The tool offers no mode or borrowing, since either would be refused.
func TestTradingStrategyBacktestAssistantQueryOffersNoModeAndNoBorrowing(t *testing.T) {
	fixture := newTradingStrategyBacktestAssistantQueryUnderTest(t)

	argumentSchema := fixture.backtestAssistantQuery.ArgumentSchema()
	description := fixture.backtestAssistantQuery.Description()

	assert.NotContains(t, argumentSchema, "tradingMode")
	assert.NotContains(t, argumentSchema, "leverage")
	assert.NotContains(t, argumentSchema, "maintenanceMarginRate")
	assert.Contains(t, argumentSchema, `"additionalProperties":false`)

	// The description says the replay is spot only, so a request to short gets an answer instead of a substituted setting.
	assert.Contains(t, description, "只做現貨")
	assert.Contains(t, description, "沒有交易模式可以給")
	assert.NotContains(t, description, "liquidationExitCount")
}

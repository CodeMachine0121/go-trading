package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// replayStart is where every replay below begins; candles run one hour apart so a
// point on the curve is easy to name.
var replayStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

const flatSignal = 0.0
const buySignal = 1.0
const sellSignal = -1.0

// replayedCandleAt builds the nth candle of a replay closing at that price.
func replayedCandleAt(candleIndex int, closePrice float64) vo.KCandleVo {
	return vo.KCandleVo{
		Symbol:              "BTCUSDT",
		OpenTimeUnixSeconds: replayStart.Add(time.Duration(candleIndex) * time.Hour).Unix(),
		Close:               closePrice,
	}
}

// replayOf walks a strategy over candles priced by closePrices, acting on the signal
// standing at the same position. The two lists are always the same length here, which
// is what the script runner guarantees.
func replayOf(
	t *testing.T,
	initialCapital int64,
	sizingMode string,
	sizingValue int64,
	closePrices []float64,
	signalStrengths []float64,
) domains.BacktestSimulationDomain {
	t.Helper()

	positionSizing, err := domains.NewPositionSizingDomain(
		sizingMode, decimal.NewFromInt(sizingValue))
	require.NoError(t, err)

	inputKCandles := make([]vo.KCandleVo, 0, len(closePrices))
	for candleIndex, closePrice := range closePrices {
		inputKCandles = append(inputKCandles, replayedCandleAt(candleIndex, closePrice))
	}

	perCandleIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(signalStrengths))
	for _, signalStrength := range signalStrengths {
		perCandleIndicatorValues = append(perCandleIndicatorValues, signalResultOf(signalStrength))
	}

	return domains.NewBacktestSimulationDomain(
		decimal.NewFromInt(initialCapital), positionSizing, inputKCandles, perCandleIndicatorValues)
}

func TestBacktestSimulationHoldsOnePositionAtATime(t *testing.T) {
	t.Run("buying while flat opens a long at that candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			[]float64{buySignal, flatSignal}).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		// Bought 100 units at 100; at 110 the account is worth 11,000.
		assert.True(t, decimal.NewFromInt(11000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("selling while flat opens a short at that candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 90},
			[]float64{sellSignal, flatSignal}).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		// A short of 100 units entered at 100 is worth 11,000 once the price is 90.
		assert.True(t, decimal.NewFromInt(11000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("buying again while long changes nothing", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			[]float64{buySignal, buySignal, flatSignal}).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		// Still the original 100 units: 100 x 120.
		assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("selling again while short changes nothing", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 90, 80},
			[]float64{sellSignal, sellSignal, flatSignal}).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
	})

	t.Run("buying while short reverses on the same candle at the same price", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{200, 100, 110},
			[]float64{sellSignal, buySignal, flatSignal}).ToDto()

		require.Len(t, result.ClosedTrades, 1)
		closedTrade := result.ClosedTrades[0]
		assert.Equal(t, string(vo.PositionDirectionShort), closedTrade.Direction)
		assert.True(t, decimal.NewFromInt(100).Equal(closedTrade.ExitPrice))
		// The short entered at 200 with 50 units and exited at 100: it made 5,000.
		assert.True(t, decimal.NewFromInt(5000).Equal(closedTrade.Profit),
			"profit was %s", closedTrade.Profit)
		// Both openings count, and the long entered at that very same 100.
		assert.Equal(t, 2, result.Summary.PositionOpenCount)
		// 15,000 staked at 100 is 150 units, worth 16,500 at 110.
		assert.True(t, decimal.NewFromInt(16500).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("selling while long reverses on the same candle at the same price", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 100, 90},
			[]float64{buySignal, sellSignal, flatSignal}).ToDto()

		require.Len(t, result.ClosedTrades, 1)
		assert.Equal(t, string(vo.PositionDirectionLong), result.ClosedTrades[0].Direction)
		assert.True(t, decimal.NewFromInt(100).Equal(result.ClosedTrades[0].ExitPrice))
		assert.Equal(t, 2, result.Summary.PositionOpenCount)
	})

	t.Run("staying flat does nothing at all", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			[]float64{flatSignal, flatSignal, flatSignal}).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
	})

	t.Run("a script that never names a signal trades nothing", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
		require.NoError(t, err)
		inputKCandles := []vo.KCandleVo{replayedCandleAt(0, 100), replayedCandleAt(1, 110)}
		movingAverageOnly := []map[string]vo.IndicatorValueVo{
			{"ma": {Numbers: []float64{100}}},
			{"ma": {Numbers: []float64{105}}},
		}

		result := domains.NewBacktestSimulationDomain(
			decimal.NewFromInt(10000), positionSizing, inputKCandles, movingAverageOnly).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity))
	})
}

func TestBacktestSimulationStakesWhatTheSizingModeSays(t *testing.T) {
	t.Run("a percentage leaves the rest of the account alone", func(t *testing.T) {
		result := replayOf(t, 10000, "percentage", 50,
			[]float64{100, 110},
			[]float64{buySignal, flatSignal}).ToDto()

		// 5,000 staked buys 50 units; at 110 they are worth 5,500, plus 5,000 untouched.
		assert.True(t, decimal.NewFromInt(10500).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("a fixed amount stakes the same figure", func(t *testing.T) {
		result := replayOf(t, 10000, "fixedAmount", 3000,
			[]float64{100, 110},
			[]float64{buySignal, flatSignal}).ToDto()

		// 3,000 staked buys 30 units, worth 3,300 at 110, plus 7,000 untouched.
		assert.True(t, decimal.NewFromInt(10300).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
		assert.Equal(t, 1, result.Summary.PositionOpenCount)
	})

	t.Run("a fixed amount the account cannot cover skips the opening", func(t *testing.T) {
		result := replayOf(t, 2000, "fixedAmount", 3000,
			[]float64{100, 110},
			[]float64{buySignal, buySignal}).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		assert.True(t, decimal.NewFromInt(2000).Equal(result.Summary.FinalEquity))
		// Skipping is not failing: the curve still has a point per candle.
		assert.Len(t, result.EquityCurve, 2)
	})

	t.Run("a fixed amount above the starting capital skips every opening", func(t *testing.T) {
		result := replayOf(t, 10000, "fixedAmount", 30000,
			[]float64{100, 110, 90},
			[]float64{buySignal, sellSignal, buySignal}).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity))
	})
}

func TestBacktestSimulationEquityCurve(t *testing.T) {
	t.Run("the money does not move while the strategy does not", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			[]float64{flatSignal, flatSignal, flatSignal}).ToDto()

		require.Len(t, result.EquityCurve, 3)
		for _, equityPoint := range result.EquityCurve {
			assert.True(t, decimal.NewFromInt(10000).Equal(equityPoint.Equity),
				"point was %s", equityPoint.Equity)
		}
	})

	t.Run("an open position is valued at each candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			[]float64{buySignal, flatSignal}).ToDto()

		require.Len(t, result.EquityCurve, 2)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.EquityCurve[0].Equity))
		assert.True(t, decimal.NewFromInt(11000).Equal(result.EquityCurve[1].Equity),
			"point was %s", result.EquityCurve[1].Equity)
	})

	t.Run("one candle makes one point, carrying that candle's start", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120, 130, 140},
			[]float64{flatSignal, flatSignal, flatSignal, flatSignal, flatSignal}).ToDto()

		require.Len(t, result.EquityCurve, 5)
		assert.Equal(t, 5, result.UsedCandleCount)
		for candleIndex, equityPoint := range result.EquityCurve {
			assert.Equal(t,
				replayStart.Add(time.Duration(candleIndex)*time.Hour), equityPoint.OpenTime)
		}
	})
}

func TestBacktestSimulationReportCard(t *testing.T) {
	t.Run("the total return is measured against what it started with", func(t *testing.T) {
		// 10,000 all in at 100 is 100 units; at 125 the account is worth 12,500.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 125},
			[]float64{buySignal, flatSignal}).ToDto()

		assert.InDelta(t, 0.25, result.Summary.TotalReturnRate, 1e-9)
	})

	t.Run("the drawdown is the worst fall from a peak", func(t *testing.T) {
		// 100 units bought at 100: the curve runs 10,000 / 12,000 / 9,000 / 11,000.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 120, 90, 110},
			[]float64{buySignal, flatSignal, flatSignal, flatSignal}).ToDto()

		assert.InDelta(t, 0.25, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("a curve that only falls is measured from the starting capital", func(t *testing.T) {
		// 100 units bought at 100, then the price only falls: 9,500 then 9,000.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 95, 90},
			[]float64{buySignal, flatSignal, flatSignal}).ToDto()

		assert.InDelta(t, 0.10, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("a replay that never traded has no drawdown", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 50},
			[]float64{flatSignal, flatSignal}).ToDto()

		assert.InDelta(t, 0.0, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("the win rate counts only the trades that made money", func(t *testing.T) {
		// Every reversal closes one round trip and opens the next, so five alternating
		// opinions leave four finished trades: long 100 to 110, short 110 to 100,
		// long 100 to 120, short 120 to 130. Only the last one lost.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 100, 120, 130},
			[]float64{buySignal, sellSignal, buySignal, sellSignal, buySignal}).ToDto()

		require.Len(t, result.ClosedTrades, 4)
		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 0.75, *result.Summary.WinRate, 1e-9)
	})

	t.Run("a trade that broke even does not count as a win", func(t *testing.T) {
		// Long 100 to 110 makes money; the short it reversed into goes out at the very
		// price it came in at.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 110},
			[]float64{buySignal, sellSignal, buySignal}).ToDto()

		require.Len(t, result.ClosedTrades, 2)
		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 0.5, *result.Summary.WinRate, 1e-9)
	})

	t.Run("nothing closed leaves the win rate unanswered rather than zero", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			[]float64{buySignal, buySignal}).ToDto()

		assert.Nil(t, result.Summary.WinRate)
	})

	t.Run("a position still open counts as an opening but not as a trade", func(t *testing.T) {
		// Long, reverse to short, reverse to long — the last one never closes.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 120, 120},
			[]float64{buySignal, sellSignal, buySignal, flatSignal}).ToDto()

		assert.Equal(t, 3, result.Summary.PositionOpenCount)
		assert.Len(t, result.ClosedTrades, 2)
	})

	t.Run("what is left includes the position still open", func(t *testing.T) {
		// 10,000 all in at 100 is 100 units, unclosed at 120.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 120},
			[]float64{buySignal, buySignal}).ToDto()

		assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
		assert.Empty(t, result.ClosedTrades)
	})

	t.Run("every closed trade reports both ends and what it made", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 110},
			[]float64{buySignal, sellSignal, flatSignal}).ToDto()

		require.Len(t, result.ClosedTrades, 1)
		closedTrade := result.ClosedTrades[0]
		assert.Equal(t, string(vo.PositionDirectionLong), closedTrade.Direction)
		assert.Equal(t, replayStart, closedTrade.EntryTime)
		assert.True(t, decimal.NewFromInt(100).Equal(closedTrade.EntryPrice))
		assert.Equal(t, replayStart.Add(time.Hour), closedTrade.ExitTime)
		assert.True(t, decimal.NewFromInt(110).Equal(closedTrade.ExitPrice))
		assert.True(t, decimal.NewFromInt(1000).Equal(closedTrade.Profit),
			"profit was %s", closedTrade.Profit)
	})

	t.Run("the starting capital is reported back as it was given", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			[]float64{flatSignal, flatSignal}).ToDto()

		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.InitialCapital))
	})
}

func TestBacktestSimulationWithMissingSignals(t *testing.T) {
	t.Run("a candle with no result of its own is read as flat", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
		require.NoError(t, err)
		inputKCandles := []vo.KCandleVo{
			replayedCandleAt(0, 100), replayedCandleAt(1, 110), replayedCandleAt(2, 120),
		}

		result := domains.NewBacktestSimulationDomain(
			decimal.NewFromInt(10000), positionSizing, inputKCandles,
			[]map[string]vo.IndicatorValueVo{signalResultOf(buySignal)}).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Len(t, result.EquityCurve, 3)
		assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity))
	})
}

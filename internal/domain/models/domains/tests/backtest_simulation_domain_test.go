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

const (
	holdSignal = vo.SignalHold
	buySignal  = vo.SignalBuy
	sellSignal = vo.SignalSell
)

// replayedCandleAt builds the nth candle of a replay closing at that price.
func replayedCandleAt(candleIndex int, closePrice float64) vo.KCandleVo {
	return vo.KCandleVo{
		Symbol:              "BTCUSDT",
		OpenTimeUnixSeconds: replayStart.Add(time.Duration(candleIndex) * time.Hour).Unix(),
		Close:               closePrice,
	}
}

// replayOf walks a strategy script over candles priced by closePrices, acting on the signal
// standing at the same position. The two lists are always the same length here, which
// is what the script runner guarantees.
func replayOf(
	t *testing.T,
	initialCapital int64,
	sizingMode string,
	sizingValue int64,
	closePrices []float64,
	signals ...vo.SignalVo,
) domains.BacktestSimulationDomain {
	t.Helper()

	positionSizing, err := domains.NewPositionSizingDomain(
		sizingMode, decimal.NewFromInt(sizingValue))
	require.NoError(t, err)

	inputKCandles := make([]vo.KCandleVo, 0, len(closePrices))
	for candleIndex, closePrice := range closePrices {
		inputKCandles = append(inputKCandles, replayedCandleAt(candleIndex, closePrice))
	}

	return domains.NewBacktestSimulationDomain(
		decimal.NewFromInt(initialCapital),
		domains.NewBacktestPositionTermsDomain(positionSizing,
			domains.BacktestExitLevelsDomain{},
			domains.BacktestTransactionCostsDomain{}),
		domains.BacktestFillTimingDomain{},
		inputKCandles, signalDomainsSaying(signals...))
}

func TestBacktestSimulationHoldsOnePositionAtATime(t *testing.T) {
	t.Run("buying while flat opens a long at that candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			buySignal, holdSignal).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		// Bought 100 units at 100; at 110 the account is worth 11,000.
		assert.True(t, decimal.NewFromInt(11000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("selling while flat does nothing at all", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 90},
			sellSignal, holdSignal).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		// There was nothing to sell, so the account never left cash.
		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("buying again while long changes nothing", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			buySignal, buySignal, holdSignal).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		// Still the original 100 units: 100 x 120.
		assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("selling while long closes it back to cash at that candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 100, 90},
			buySignal, sellSignal, holdSignal).ToDto()

		require.Len(t, result.ClosedTrades, 1)
		assert.Equal(t, string(vo.PositionDirectionLong), result.ClosedTrades[0].Direction)
		assert.True(t, decimal.NewFromInt(100).Equal(result.ClosedTrades[0].ExitPrice))
		// One opening, not two: the sale went to cash rather than out the other way,
		// so the fall to 90 happened to somebody else.
		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("holding does nothing at all", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			holdSignal, holdSignal, holdSignal).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
	})
}

func TestBacktestSimulationStakesWhatTheSizingModeSays(t *testing.T) {
	t.Run("a percentage leaves the rest of the account alone", func(t *testing.T) {
		result := replayOf(t, 10000, "percentage", 50,
			[]float64{100, 110},
			buySignal, holdSignal).ToDto()

		// 5,000 staked buys 50 units; at 110 they are worth 5,500, plus 5,000 untouched.
		assert.True(t, decimal.NewFromInt(10500).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("a fixed amount stakes the same figure", func(t *testing.T) {
		result := replayOf(t, 10000, "fixedAmount", 3000,
			[]float64{100, 110},
			buySignal, holdSignal).ToDto()

		// 3,000 staked buys 30 units, worth 3,300 at 110, plus 7,000 untouched.
		assert.True(t, decimal.NewFromInt(10300).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
		assert.Equal(t, 1, result.Summary.PositionOpenCount)
	})

	t.Run("a fixed amount the account cannot cover skips the opening", func(t *testing.T) {
		result := replayOf(t, 2000, "fixedAmount", 3000,
			[]float64{100, 110},
			buySignal, buySignal).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		assert.True(t, decimal.NewFromInt(2000).Equal(result.Summary.FinalEquity))
		// Skipping is not failing: the curve still has a point per candle.
		assert.Len(t, result.EquityCurve, 2)
	})

	t.Run("a fixed amount above the starting capital skips every opening", func(t *testing.T) {
		result := replayOf(t, 10000, "fixedAmount", 30000,
			[]float64{100, 110, 90},
			buySignal, sellSignal, buySignal).ToDto()

		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.FinalEquity))
	})
}

func TestBacktestSimulationEquityCurve(t *testing.T) {
	t.Run("the money does not move while the strategy script does not", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120},
			holdSignal, holdSignal, holdSignal).ToDto()

		require.Len(t, result.EquityCurve, 3)
		for _, equityPoint := range result.EquityCurve {
			assert.True(t, decimal.NewFromInt(10000).Equal(equityPoint.Equity),
				"point was %s", equityPoint.Equity)
		}
	})

	t.Run("an open position is valued at each candle's close", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			buySignal, holdSignal).ToDto()

		require.Len(t, result.EquityCurve, 2)
		assert.True(t, decimal.NewFromInt(10000).Equal(result.EquityCurve[0].Equity))
		assert.True(t, decimal.NewFromInt(11000).Equal(result.EquityCurve[1].Equity),
			"point was %s", result.EquityCurve[1].Equity)
	})

	t.Run("one candle makes one point, carrying that candle's start", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 120, 130, 140},
			holdSignal, holdSignal, holdSignal, holdSignal, holdSignal).ToDto()

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
			buySignal, holdSignal).ToDto()

		assert.InDelta(t, 0.25, result.Summary.TotalReturnRate, 1e-9)
	})

	t.Run("the drawdown is the worst fall from a peak", func(t *testing.T) {
		// 100 units bought at 100: the curve runs 10,000 / 12,000 / 9,000 / 11,000.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 120, 90, 110},
			buySignal, holdSignal, holdSignal, holdSignal).ToDto()

		assert.InDelta(t, 0.25, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("a curve that only falls is measured from the starting capital", func(t *testing.T) {
		// 100 units bought at 100, then the price only falls: 9,500 then 9,000.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 95, 90},
			buySignal, holdSignal, holdSignal).ToDto()

		assert.InDelta(t, 0.10, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("a replay that never traded has no drawdown", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 50},
			holdSignal, holdSignal).ToDto()

		assert.InDelta(t, 0.0, result.Summary.MaximumDrawdown, 1e-9)
	})

	t.Run("the win rate counts only the trades that made money", func(t *testing.T) {
		// Each sell closes one round trip and each buy opens the next, so six
		// alternating opinions leave three finished trades: 100 to 110, 100 to 120
		// and 130 to 120. Only the last one lost.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 100, 120, 130, 120},
			buySignal, sellSignal, buySignal, sellSignal, buySignal, sellSignal).ToDto()

		require.Len(t, result.ClosedTrades, 3)
		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 2.0/3.0, *result.Summary.WinRate, 1e-9)
	})

	t.Run("a trade that broke even does not count as a win", func(t *testing.T) {
		// Bought at 100 and sold at 110 makes money; bought back at 110 and sold at
		// that very same price gives nothing back.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 110, 110},
			buySignal, sellSignal, buySignal, sellSignal).ToDto()

		require.Len(t, result.ClosedTrades, 2)
		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 0.5, *result.Summary.WinRate, 1e-9)
	})

	t.Run("nothing closed leaves the win rate unanswered rather than zero", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110},
			buySignal, buySignal).ToDto()

		assert.Nil(t, result.Summary.WinRate)
	})

	t.Run("a position still open counts as an opening but not as a trade", func(t *testing.T) {
		// Buy, sell, buy again — the last one never closes.
		result := replayOf(t, 10000, "percentage", 10,
			[]float64{100, 110, 120, 120},
			buySignal, sellSignal, buySignal, holdSignal).ToDto()

		assert.Equal(t, 2, result.Summary.PositionOpenCount)
		assert.Len(t, result.ClosedTrades, 1)
	})

	t.Run("what is left includes the position still open", func(t *testing.T) {
		// 10,000 all in at 100 is 100 units, unclosed at 120.
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 120},
			buySignal, buySignal).ToDto()

		assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
		assert.Empty(t, result.ClosedTrades)
	})

	t.Run("every closed trade reports both ends and what it made", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 110, 110},
			buySignal, sellSignal, holdSignal).ToDto()

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
			holdSignal, holdSignal).ToDto()

		assert.True(t, decimal.NewFromInt(10000).Equal(result.Summary.InitialCapital))
	})
}

// One walk over three candles, read in full: it buys, it sells into cash, and the fall
// afterwards happens to somebody else.
func TestBacktestSimulationSitsInCashAfterSelling(t *testing.T) {
	result := replayOf(t, 10000, "allIn", 0,
		[]float64{100, 120, 90}, buySignal, sellSignal, holdSignal).ToDto()

	// Bought at 100, sold at 120, and the drop to 90 happened to somebody else.
	assert.True(t, decimal.NewFromInt(12000).Equal(result.Summary.FinalEquity),
		"final equity was %s", result.Summary.FinalEquity)
	assert.Equal(t, 1, result.Summary.PositionOpenCount)
	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionLong), result.ClosedTrades[0].Direction)
	assert.True(t, decimal.NewFromInt(2000).Equal(result.ClosedTrades[0].Profit))
	// The last point sits still because the account is holding cash, not a bet.
	require.Len(t, result.EquityCurve, 3)
	assert.True(t, decimal.NewFromInt(12000).Equal(result.EquityCurve[1].Equity))
	assert.True(t, decimal.NewFromInt(12000).Equal(result.EquityCurve[2].Equity))
}

// Three things a replay does that only show up over a whole stretch rather than on
// one candle.
func TestBacktestSimulationOverAWholeStretch(t *testing.T) {
	t.Run("a stretch that never buys finishes with nothing having happened", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 90, 80},
			sellSignal, sellSignal, holdSignal).ToDto()

		// Nothing to sell, and no way to short: the account simply sat there. This is
		// a legitimate outcome, not a failure.
		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
		require.Len(t, result.EquityCurve, 3)
		for _, equityPoint := range result.EquityCurve {
			assert.True(t, decimal.NewFromInt(10000).Equal(equityPoint.Equity),
				"a point on the curve was %s", equityPoint.Equity)
		}
	})

	t.Run("a fixed stake it cannot cover after a sale skips that opening", func(t *testing.T) {
		// Stakes 8,000 a time. The first buy fits; the sale returns only 4,000, so the
		// next buy cannot be placed and the replay carries on in cash.
		result := replayOf(t, 10000, "fixedAmount", 8000,
			[]float64{100, 50, 60},
			buySignal, sellSignal, buySignal).ToDto()

		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		require.Len(t, result.ClosedTrades, 1)
		// 2,000 never staked, plus the 4,000 the sale returned.
		assert.True(t, decimal.NewFromInt(6000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("a long still open at the end counts but is not a round trip", func(t *testing.T) {
		result := replayOf(t, 10000, "allIn", 0,
			[]float64{100, 120, 150},
			buySignal, holdSignal, holdSignal).ToDto()

		// Bought 100 units at 100 and never sold: worth 15,000 at the last close.
		assert.True(t, decimal.NewFromInt(15000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		assert.Empty(t, result.ClosedTrades)
	})
}

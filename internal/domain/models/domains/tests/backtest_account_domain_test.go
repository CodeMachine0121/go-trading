package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// accountStakingEverything opens an account that puts all its cash on every bet.
func accountStakingEverything(t *testing.T, initialCapital int64) *domains.BacktestAccountDomain {
	t.Helper()

	positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
	require.NoError(t, err)

	return domains.NewBacktestAccountDomain(decimal.NewFromInt(initialCapital), positionSizing)
}

// signalOf is one candle's opinion, built from the strength a script would have named.
func signalOf(signalStrength float64) domains.SignalDomain {
	return domains.NewSignalDomain(signalResultOf(signalStrength))
}

func TestBacktestAccountDomainApply(t *testing.T) {
	t.Run("an untouched account is worth exactly what it started with", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(999))))
		assert.Equal(t, 0, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
	})

	t.Run("a flat opinion moves nothing", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(0), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("a repeated opinion is heard once", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(1), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(1), positionExitTime, decimal.NewFromInt(200))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		// Still the 100 units bought at 100, now worth 20,000.
		assert.True(t, decimal.NewFromInt(20000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("a reversal closes one bet and places the other at the same price", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(1), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(-1), positionExitTime, decimal.NewFromInt(110))

		require.Len(t, account.ClosedTradeDtos(), 1)
		assert.Equal(t, string(vo.PositionDirectionLong), account.ClosedTradeDtos()[0].Direction)
		assert.True(t, decimal.NewFromInt(110).Equal(account.ClosedTradeDtos()[0].ExitPrice))
		assert.Equal(t, 2, account.PositionOpenCount())
		// The whole 11,000 went back out as a short at that very same 110.
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(110))))
	})

	t.Run("an opening the account cannot afford leaves it flat", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain(
			"fixedAmount", decimal.NewFromInt(3000))
		require.NoError(t, err)
		account := domains.NewBacktestAccountDomain(decimal.NewFromInt(2000), positionSizing)

		account.Apply(signalOf(1), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(2000).Equal(account.EquityAt(decimal.NewFromInt(999))))
	})

	t.Run("a market priced at nothing opens nothing", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(1), positionEntryTime, decimal.Zero)

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(100))))
	})
}

func TestBacktestAccountDomainWinRate(t *testing.T) {
	t.Run("nothing closed leaves the rate unanswered", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)
		account.Apply(signalOf(1), positionEntryTime, decimal.NewFromInt(100))

		_, isApplicable := account.WinRate()

		assert.False(t, isApplicable)
	})

	t.Run("the rate counts only the round trips that made money", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)
		// Long 100 to 110 makes money; the short it reverses into goes out where it
		// came in.
		account.Apply(signalOf(1), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(-1), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(1), positionExitTime, decimal.NewFromInt(110))

		winRate, isApplicable := account.WinRate()

		require.True(t, isApplicable)
		assert.Len(t, account.ClosedTradeDtos(), 2)
		assert.InDelta(t, 0.5, winRate, 1e-9)
	})
}

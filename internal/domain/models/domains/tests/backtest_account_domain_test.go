package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accountStakingEverything(t *testing.T, initialCapital int64) *domains.BacktestAccountDomain {
	t.Helper()

	positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
	require.NoError(t, err)

	return domains.NewBacktestAccountDomain(
		decimal.NewFromInt(initialCapital),
		domains.NewBacktestPositionTermsDomain(positionSizing,
			domains.BacktestExitLevelsDomain{},
			domains.BacktestTransactionCostsDomain{}))
}

func signalOf(signal vo.SignalVo) domains.SignalDomain {
	return domains.NewSignalDomain(signalResultOf(signal))
}

func TestBacktestAccountDomainApply(t *testing.T) {
	t.Run("an untouched account is worth exactly what it started with", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(999))))
		assert.Equal(t, 0, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
	})

	t.Run("a hold opinion moves nothing", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalHold), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("a repeated opinion is heard once", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(200))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		// Still the 100 units bought at 100.
		assert.True(t, decimal.NewFromInt(20000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("an opening the account cannot afford leaves it flat", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain(
			"fixedAmount", decimal.NewFromInt(3000))
		require.NoError(t, err)
		account := domains.NewBacktestAccountDomain(
			decimal.NewFromInt(2000),
			domains.NewBacktestPositionTermsDomain(positionSizing,
				domains.BacktestExitLevelsDomain{},
				domains.BacktestTransactionCostsDomain{}))

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(2000).Equal(account.EquityAt(decimal.NewFromInt(999))))
	})

	t.Run("a market priced at nothing opens nothing", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.Zero)

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(100))))
	})
}

func TestBacktestAccountDomainWinRate(t *testing.T) {
	t.Run("nothing closed leaves the rate unanswered", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)
		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		_, isApplicable := account.WinRate()

		assert.False(t, isApplicable)
	})

	t.Run("the rate counts only the round trips that made money", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)
		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(100))

		winRate, isApplicable := account.WinRate()

		require.True(t, isApplicable)
		assert.Len(t, account.ClosedTradeDtos(), 2)
		assert.InDelta(t, 0.5, winRate, 1e-9)
	})
}

// A replay only goes long: a sell moves to cash, and a sell while already in cash does nothing.
func TestBacktestAccountDomainTradingSpot(t *testing.T) {
	t.Run("buying while flat opens a long", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(100))))
	})

	t.Run("buying again is heard once", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(200))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		// Still the 100 units bought at 100.
		assert.True(t, decimal.NewFromInt(20000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("selling a long closes it back to cash", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))

		require.Len(t, account.ClosedTradeDtos(), 1)
		closedTrade := account.ClosedTradeDtos()[0]
		assert.Equal(t, string(vo.PositionDirectionLong), closedTrade.Direction)
		assert.True(t, decimal.NewFromInt(100).Equal(closedTrade.EntryPrice))
		assert.True(t, decimal.NewFromInt(110).Equal(closedTrade.ExitPrice))
		assert.True(t, decimal.NewFromInt(1000).Equal(closedTrade.Profit))
		// The sell opened nothing in its place.
		assert.Equal(t, 1, account.PositionOpenCount())
	})

	t.Run("what a closed spot account holds does not move with the price", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))

		// Sitting in cash is immune to further price moves.
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(90))))
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(500))))
	})

	t.Run("selling with nothing to sell does nothing at all", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(900))))
	})

	t.Run("selling twice closes once", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(90))

		assert.Len(t, account.ClosedTradeDtos(), 1)
		assert.Equal(t, 1, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(90))))
	})

	t.Run("buying back after a sale opens a second long", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(120))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(90))

		assert.Equal(t, 2, account.PositionOpenCount())
		require.Len(t, account.ClosedTradeDtos(), 1)
		assert.True(t, decimal.NewFromInt(2000).Equal(account.ClosedTradeDtos()[0].Profit))
		for _, closedTrade := range account.ClosedTradeDtos() {
			assert.Equal(t, string(vo.PositionDirectionLong), closedTrade.Direction)
		}
	})

	t.Run("an opening it cannot afford still leaves it flat", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain(
			"fixedAmount", decimal.NewFromInt(3000))
		require.NoError(t, err)
		account := domains.NewBacktestAccountDomain(
			decimal.NewFromInt(2000),
			domains.NewBacktestPositionTermsDomain(positionSizing,
				domains.BacktestExitLevelsDomain{},
				domains.BacktestTransactionCostsDomain{}))

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(2000).Equal(account.EquityAt(decimal.NewFromInt(999))))
	})

	t.Run("holding leaves an open long alone", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalHold), positionExitTime, decimal.NewFromInt(110))

		assert.Empty(t, account.ClosedTradeDtos())
		assert.Equal(t, 1, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(110))))
	})
}

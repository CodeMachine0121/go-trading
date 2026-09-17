package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tradingModeOf reads a declared trading mode, failing the test when it is one the
// replay would have refused.
func tradingModeOf(t *testing.T, declaredMode string) domains.TradingModeDomain {
	t.Helper()

	tradingMode, err := domains.NewTradingModeDomain(declaredMode)
	require.NoError(t, err)

	return tradingMode
}

// accountStakingEverything opens an account that puts all its cash on every bet, and
// trades the way a replay always has: always in the market.
func accountStakingEverything(t *testing.T, initialCapital int64) *domains.BacktestAccountDomain {
	t.Helper()

	return accountStakingEverythingIn(t, initialCapital, "longShort")
}

// accountStakingEverythingIn is the same account under a named trading mode.
func accountStakingEverythingIn(
	t *testing.T, initialCapital int64, declaredMode string,
) *domains.BacktestAccountDomain {
	t.Helper()

	positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
	require.NoError(t, err)

	return domains.NewBacktestAccountDomain(
		decimal.NewFromInt(initialCapital), positionSizing, tradingModeOf(t, declaredMode))
}

// signalOf is one candle's opinion.
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
		// Still the 100 units bought at 100, now worth 20,000.
		assert.True(t, decimal.NewFromInt(20000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("a reversal closes one bet and places the other at the same price", func(t *testing.T) {
		account := accountStakingEverything(t, 10000)

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))

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
		account := domains.NewBacktestAccountDomain(
			decimal.NewFromInt(2000), positionSizing, tradingModeOf(t, "longShort"))

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
		// Long 100 to 110 makes money; the short it reverses into goes out where it
		// came in.
		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(110))

		winRate, isApplicable := account.WinRate()

		require.True(t, isApplicable)
		assert.Len(t, account.ClosedTradeDtos(), 2)
		assert.InDelta(t, 0.5, winRate, 1e-9)
	})
}

// A spot account only ever goes long: a sell gets it out into cash, and there is no
// such thing as a sell while it is already there.
func TestBacktestAccountDomainTradingSpot(t *testing.T) {
	t.Run("buying while flat opens a long", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(100))))
	})

	t.Run("buying again is heard once", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(200))

		assert.Equal(t, 1, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		// Still the 100 units bought at 100, now worth 20,000.
		assert.True(t, decimal.NewFromInt(20000).Equal(account.EquityAt(decimal.NewFromInt(200))))
	})

	t.Run("selling a long closes it back to cash", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))

		require.Len(t, account.ClosedTradeDtos(), 1)
		closedTrade := account.ClosedTradeDtos()[0]
		assert.Equal(t, string(vo.PositionDirectionLong), closedTrade.Direction)
		assert.True(t, decimal.NewFromInt(100).Equal(closedTrade.EntryPrice))
		assert.True(t, decimal.NewFromInt(110).Equal(closedTrade.ExitPrice))
		assert.True(t, decimal.NewFromInt(1000).Equal(closedTrade.Profit))
		// Nothing was opened in its place, so the sell cost one opening, not two.
		assert.Equal(t, 1, account.PositionOpenCount())
	})

	t.Run("what a closed spot account holds does not move with the price", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))

		// 11,000 in cash, whatever the market does next. This is the whole point of
		// the mode: standing aside is a state the account can actually be in.
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(90))))
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(500))))
	})

	t.Run("selling with nothing to sell does nothing at all", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.Empty(t, account.ClosedTradeDtos())
		assert.True(t, decimal.NewFromInt(10000).Equal(account.EquityAt(decimal.NewFromInt(900))))
	})

	t.Run("selling twice closes once", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(110))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(90))

		assert.Len(t, account.ClosedTradeDtos(), 1)
		assert.Equal(t, 1, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(90))))
	})

	t.Run("buying back after a sale opens a second long", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalSell), positionExitTime, decimal.NewFromInt(120))
		account.Apply(signalOf(vo.SignalBuy), positionExitTime, decimal.NewFromInt(90))

		assert.Equal(t, 2, account.PositionOpenCount())
		require.Len(t, account.ClosedTradeDtos(), 1)
		assert.True(t, decimal.NewFromInt(2000).Equal(account.ClosedTradeDtos()[0].Profit))
		// Every round trip a spot account ever finishes faces the same way.
		for _, closedTrade := range account.ClosedTradeDtos() {
			assert.Equal(t, string(vo.PositionDirectionLong), closedTrade.Direction)
		}
	})

	t.Run("an opening it cannot afford still leaves it flat", func(t *testing.T) {
		positionSizing, err := domains.NewPositionSizingDomain(
			"fixedAmount", decimal.NewFromInt(3000))
		require.NoError(t, err)
		account := domains.NewBacktestAccountDomain(
			decimal.NewFromInt(2000), positionSizing, tradingModeOf(t, "spot"))

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))

		assert.Equal(t, 0, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(2000).Equal(account.EquityAt(decimal.NewFromInt(999))))
	})

	t.Run("holding leaves an open long alone", func(t *testing.T) {
		account := accountStakingEverythingIn(t, 10000, "spot")

		account.Apply(signalOf(vo.SignalBuy), positionEntryTime, decimal.NewFromInt(100))
		account.Apply(signalOf(vo.SignalHold), positionExitTime, decimal.NewFromInt(110))

		assert.Empty(t, account.ClosedTradeDtos())
		assert.Equal(t, 1, account.PositionOpenCount())
		assert.True(t, decimal.NewFromInt(11000).Equal(account.EquityAt(decimal.NewFromInt(110))))
	})
}

// Selling while flat is where the two modes part company, and it is the one line that
// says a spot replay can never take the short side.
func TestBacktestAccountDomainSellingWhileFlatSplitsTheModes(t *testing.T) {
	longShortAccount := accountStakingEverythingIn(t, 10000, "longShort")
	spotAccount := accountStakingEverythingIn(t, 10000, "spot")

	longShortAccount.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))
	spotAccount.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))

	assert.Equal(t, 1, longShortAccount.PositionOpenCount())
	assert.Equal(t, 0, spotAccount.PositionOpenCount())
}

package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradingModeDomainReadsWhatWasDeclared(t *testing.T) {
	testCases := []struct {
		name         string
		declaredMode string
		expectedMode vo.TradingModeVo
	}{
		{
			name:         "always in the market, spelled out",
			declaredMode: "longShort",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			name:         "long only, spelled out",
			declaredMode: "spot",
			expectedMode: vo.TradingModeSpot,
		},
		{
			name:         "long only but borrowing, spelled out",
			declaredMode: "leveragedLong",
			expectedMode: vo.TradingModeLeveragedLong,
		},
		{
			name:         "declaring nothing trades the way a replay always has",
			declaredMode: "",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			name:         "blank is declaring nothing",
			declaredMode: "   ",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			name:         "the spelling is read however it was typed",
			declaredMode: "SPOT",
			expectedMode: vo.TradingModeSpot,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingMode, err := domains.NewTradingModeDomain(testCase.declaredMode)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedMode, tradingMode.Value())
		})
	}
}

func TestTradingModeDomainRefusesWhatItCannotRead(t *testing.T) {
	_, err := domains.NewTradingModeDomain("dayTrade")

	require.Error(t, err)
	// The refusal carries the sentence alone, with no sentinel of its own. Two worlds
	// ask this question — a replay and a set of rules being saved — and each wraps the
	// sentence in the sentinel its own controller already maps. That a replay's refusal
	// names the input is asserted where a replay is: backtest_domain_test.go.
	// Both spellings are offered back; a refusal that does not say what is allowed
	// leaves the caller guessing at a string.
	assert.Contains(t, err.Error(), string(vo.TradingModeLongShort))
	assert.Contains(t, err.Error(), string(vo.TradingModeSpot))
	assert.Contains(t, err.Error(), string(vo.TradingModeLeveragedLong))
}

func TestTradingModeDomainTargetFor(t *testing.T) {
	testCases := []struct {
		name           string
		declaredMode   string
		signal         vo.SignalVo
		expectedTarget vo.TargetPositionVo
	}{
		{
			name:           "always in the market: buying asks to be long",
			declaredMode:   "longShort",
			signal:         vo.SignalBuy,
			expectedTarget: vo.TargetPositionLong,
		},
		{
			name:           "always in the market: selling asks to be short",
			declaredMode:   "longShort",
			signal:         vo.SignalSell,
			expectedTarget: vo.TargetPositionShort,
		},
		{
			name:           "always in the market: holding asks for nothing",
			declaredMode:   "longShort",
			signal:         vo.SignalHold,
			expectedTarget: vo.TargetPositionUnchanged,
		},
		{
			name:           "long only: buying asks to be long",
			declaredMode:   "spot",
			signal:         vo.SignalBuy,
			expectedTarget: vo.TargetPositionLong,
		},
		{
			name:           "long only: selling asks to be in cash",
			declaredMode:   "spot",
			signal:         vo.SignalSell,
			expectedTarget: vo.TargetPositionFlat,
		},
		{
			name:           "long only: holding asks for nothing",
			declaredMode:   "spot",
			signal:         vo.SignalHold,
			expectedTarget: vo.TargetPositionUnchanged,
		},
		{
			name:           "long only but borrowing: buying asks to be long",
			declaredMode:   "leveragedLong",
			signal:         vo.SignalBuy,
			expectedTarget: vo.TargetPositionLong,
		},
		{
			// Word for word what spot answers. The two differ over borrowing, not
			// over which way a position faces, so a replay that never borrows cannot
			// tell them apart — and that is the point rather than an oversight.
			name:           "long only but borrowing: selling asks to be in cash",
			declaredMode:   "leveragedLong",
			signal:         vo.SignalSell,
			expectedTarget: vo.TargetPositionFlat,
		},
		{
			name:           "long only but borrowing: holding asks for nothing",
			declaredMode:   "leveragedLong",
			signal:         vo.SignalHold,
			expectedTarget: vo.TargetPositionUnchanged,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingMode, err := domains.NewTradingModeDomain(testCase.declaredMode)
			require.NoError(t, err)

			assert.Equal(t, testCase.expectedTarget,
				tradingMode.TargetFor(signalOf(testCase.signal)))
		})
	}
}

// Asking for cash and having no opinion are two different things, and the account
// acts on them differently: one closes what is open, the other leaves it alone.
func TestTargetPositionWantedDirection(t *testing.T) {
	testCases := []struct {
		name                string
		target              vo.TargetPositionVo
		expectedDirection   vo.PositionDirectionVo
		expectsAnyDirection bool
	}{
		{
			name:                "a long target faces up",
			target:              vo.TargetPositionLong,
			expectedDirection:   vo.PositionDirectionLong,
			expectsAnyDirection: true,
		},
		{
			name:                "a short target faces down",
			target:              vo.TargetPositionShort,
			expectedDirection:   vo.PositionDirectionShort,
			expectsAnyDirection: true,
		},
		{name: "cash asks for no position", target: vo.TargetPositionFlat},
		{name: "no opinion asks for no position", target: vo.TargetPositionUnchanged},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			wantedDirection, wantsPosition := testCase.target.WantedDirection()

			assert.Equal(t, testCase.expectsAnyDirection, wantsPosition)
			if testCase.expectsAnyDirection {
				assert.Equal(t, testCase.expectedDirection, wantedDirection)
			}
		})
	}
}

// A mode this model does not recognise asks for nothing, so a replay under it makes
// no trades at all.
//
// The alternative — letting anything that is not spot fall through to a short — hands
// back a complete, entirely plausible long-short report card for a mode nobody meant
// to replay. A replay that visibly does nothing is the failure somebody notices; one
// that quietly did the wrong thing convincingly is not.
func TestTradingModeDomainAsksForNothingWhenItDoesNotRecogniseTheMode(t *testing.T) {
	// A zero value never went through the constructor — the exported account and
	// simulation models both take a mode by parameter, so one can reach them.
	unrecognizedMode := domains.TradingModeDomain{}

	assert.Equal(t, vo.TargetPositionUnchanged, unrecognizedMode.TargetFor(signalOf(vo.SignalSell)))
	assert.Equal(t, vo.TargetPositionUnchanged, unrecognizedMode.TargetFor(signalOf(vo.SignalHold)))
}

// An account handed a mode it cannot read never opens anything, rather than replaying
// the whole stretch as long-short.
func TestBacktestAccountDomainTradesNothingUnderAnUnrecognisedMode(t *testing.T) {
	positionSizing, err := domains.NewPositionSizingDomain("allIn", decimal.Zero)
	require.NoError(t, err)
	account := domains.NewBacktestAccountDomain(
		decimal.NewFromInt(10000), domains.TradingModeDomain{},
		domains.NewBacktestPositionTermsDomain(positionSizing,
			domains.BacktestExitLevelsDomain{}, domains.BacktestLeverageDomain{},
			domains.BacktestTransactionCostsDomain{}))

	account.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))

	assert.Equal(t, 0, account.PositionOpenCount())
	assert.Empty(t, account.ClosedTradeDtos())
}

func TestTradingModeDomainCanGoShort(t *testing.T) {
	testCases := []struct {
		name            string
		declaredMode    string
		expectedInWords string
		canGoShort      bool
	}{
		{
			name:            "long-short can hold a position that gains when the price falls",
			declaredMode:    "longShort",
			expectedInWords: "多空反手",
			canGoShort:      true,
		},
		{
			name:            "spot cannot",
			declaredMode:    "spot",
			expectedInWords: "現貨",
			canGoShort:      false,
		},
		{
			name:            "leveraged long cannot short either, for all that it borrows",
			declaredMode:    "leveragedLong",
			expectedInWords: "槓桿做多",
			canGoShort:      false,
		},
		{
			name:            "declaring nothing reads as long-short, and so can short",
			declaredMode:    "",
			expectedInWords: "多空反手",
			canGoShort:      true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingMode, err := domains.NewTradingModeDomain(testCase.declaredMode)

			require.NoError(t, err)
			assert.Equal(t, testCase.canGoShort, tradingMode.CanGoShort())
			assert.Equal(t, testCase.expectedInWords, tradingMode.InWords())
		})
	}
}

func TestTradingModeDomainZeroValueCannotGoShort(t *testing.T) {
	// One that never went through the constructor answers no, by the same rule
	// TargetFor answers "unchanged": a mode this does not recognise is the last place
	// to guess which way somebody should trade.
	assert.False(t, domains.TradingModeDomain{}.CanGoShort())
}

// Whether a mode may short and whether it may borrow are separate questions, and
// leveraged long is where they first give different answers.
func TestTradingModeDomainCanUseLeverage(t *testing.T) {
	testCases := []struct {
		name           string
		declaredMode   string
		canUseLeverage bool
	}{
		{
			name:           "long-short borrows",
			declaredMode:   "longShort",
			canUseLeverage: true,
		},
		{
			name:           "spot is cash for goods, so nobody lends against it",
			declaredMode:   "spot",
			canUseLeverage: false,
		},
		{
			name:           "leveraged long borrows although it never faces the other way",
			declaredMode:   "leveragedLong",
			canUseLeverage: true,
		},
		{
			name:           "declaring nothing reads as long-short, and so borrows",
			declaredMode:   "",
			canUseLeverage: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingMode, err := domains.NewTradingModeDomain(testCase.declaredMode)

			require.NoError(t, err)
			assert.Equal(t, testCase.canUseLeverage, tradingMode.CanUseLeverage())
		})
	}
}

func TestTradingModeDomainZeroValueCannotUseLeverage(t *testing.T) {
	// A mode this model does not recognise is the last place to start lending.
	assert.False(t, domains.TradingModeDomain{}.CanUseLeverage())
	require.Error(t, domains.TradingModeDomain{}.BorrowingRefusal())
}

// The refusal is a sentence rather than a boolean because two worlds say it — a
// replay handed a multiplier, and a bot being saved with one — and they have to say
// it in the same words.
func TestTradingModeDomainBorrowingRefusal(t *testing.T) {
	testCases := []struct {
		name           string
		declaredMode   string
		expectsRefusal bool
		expectedNaming string
	}{
		{
			name:         "long-short is not refused",
			declaredMode: "longShort",
		},
		{
			name:         "leveraged long is not refused",
			declaredMode: "leveragedLong",
		},
		{
			name:           "spot is refused, and the sentence names it",
			declaredMode:   "spot",
			expectsRefusal: true,
			expectedNaming: "現貨",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingMode, err := domains.NewTradingModeDomain(testCase.declaredMode)
			require.NoError(t, err)

			refusal := tradingMode.BorrowingRefusal()

			if !testCase.expectsRefusal {
				assert.NoError(t, refusal)
				return
			}

			require.Error(t, refusal)
			assert.Contains(t, refusal.Error(), testCase.expectedNaming)
		})
	}
}

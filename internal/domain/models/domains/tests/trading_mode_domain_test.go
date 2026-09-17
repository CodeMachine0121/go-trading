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
	assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	// The refusal names the input, so whoever asked can put the sentence beside the
	// box the person has to change rather than at the top of a page.
	fieldName, namesField := domains.BacktestFieldName(err)
	require.True(t, namesField)
	assert.Equal(t, domains.BacktestTradingModeField, fieldName)
	// Both spellings are offered back; a refusal that does not say what is allowed
	// leaves the caller guessing at a string.
	assert.Contains(t, err.Error(), string(vo.TradingModeLongShort))
	assert.Contains(t, err.Error(), string(vo.TradingModeSpot))
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
		decimal.NewFromInt(10000), positionSizing, domains.TradingModeDomain{})

	account.Apply(signalOf(vo.SignalSell), positionEntryTime, decimal.NewFromInt(100))

	assert.Equal(t, 0, account.PositionOpenCount())
	assert.Empty(t, account.ClosedTradeDtos())
}

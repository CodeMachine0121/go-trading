package domains_test

import (
	"math"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

// signalResultOf builds one candle's indicator result carrying that signal strength.
func signalResultOf(signalStrength float64) map[string]vo.IndicatorValueVo {
	return map[string]vo.IndicatorValueVo{
		domains.SignalIndicatorName: {Numbers: []float64{signalStrength}},
	}
}

func TestNewSignalDomain(t *testing.T) {
	testCases := []struct {
		name            string
		indicatorValues map[string]vo.IndicatorValueVo
		expectedSignal  vo.SignalVo
	}{
		{name: "one means buy", indicatorValues: signalResultOf(1), expectedSignal: vo.SignalBuy},
		{name: "minus one means sell", indicatorValues: signalResultOf(-1), expectedSignal: vo.SignalSell},
		{name: "zero means flat", indicatorValues: signalResultOf(0), expectedSignal: vo.SignalFlat},
		{
			name:            "any positive number means buy",
			indicatorValues: signalResultOf(0.5),
			expectedSignal:  vo.SignalBuy,
		},
		{
			name:            "any negative number means sell",
			indicatorValues: signalResultOf(-2),
			expectedSignal:  vo.SignalSell,
		},
		{
			name:            "a result without the signal name means flat",
			indicatorValues: map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}},
			expectedSignal:  vo.SignalFlat,
		},
		{
			name:            "an empty result means flat",
			indicatorValues: map[string]vo.IndicatorValueVo{},
			expectedSignal:  vo.SignalFlat,
		},
		{
			name:            "a signal that is not a number means flat",
			indicatorValues: signalResultOf(math.NaN()),
			expectedSignal:  vo.SignalFlat,
		},
		{
			name:            "an infinite signal means flat",
			indicatorValues: signalResultOf(math.Inf(1)),
			expectedSignal:  vo.SignalFlat,
		},
		{
			name: "a signal named with no value at all means flat",
			indicatorValues: map[string]vo.IndicatorValueVo{
				domains.SignalIndicatorName: {},
			},
			expectedSignal: vo.SignalFlat,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			signal := domains.NewSignalDomain(testCase.indicatorValues)

			assert.Equal(t, testCase.expectedSignal, signal.Value())
		})
	}
}

func TestSignalDomainWantedDirection(t *testing.T) {
	testCases := []struct {
		name                string
		signalStrength      float64
		expectedDirection   vo.PositionDirectionVo
		expectsAnyDirection bool
	}{
		{
			name:                "buy wants a long position",
			signalStrength:      1,
			expectedDirection:   vo.PositionDirectionLong,
			expectsAnyDirection: true,
		},
		{
			name:                "sell wants a short position",
			signalStrength:      -1,
			expectedDirection:   vo.PositionDirectionShort,
			expectsAnyDirection: true,
		},
		{name: "flat wants nothing", signalStrength: 0, expectsAnyDirection: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			signal := domains.NewSignalDomain(signalResultOf(testCase.signalStrength))

			wantedDirection, wantsPosition := signal.WantedDirection()

			assert.Equal(t, testCase.expectsAnyDirection, wantsPosition)
			if testCase.expectsAnyDirection {
				assert.Equal(t, testCase.expectedDirection, wantedDirection)
			}
		})
	}
}

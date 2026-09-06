package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

// signalResultOf packs one signal into a script result the way the runner does under
// the signal kind: one value, filed under the well-known key.
func signalResultOf(signal vo.SignalVo) map[string]vo.IndicatorValueVo {
	return map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: signal}}
}

// signalDomainsSaying builds one opinion per signal, in order — the shape a replay
// works in once the script runner has read each candle's signal.
func signalDomainsSaying(signals ...vo.SignalVo) []domains.SignalDomain {
	signalDomains := make([]domains.SignalDomain, 0, len(signals))
	for _, signal := range signals {
		signalDomains = append(signalDomains, domains.NewSignalDomain(signalResultOf(signal)))
	}

	return signalDomains
}

func TestNewSignalDomainCarriesTheSignalItWasGiven(t *testing.T) {
	for _, signal := range []vo.SignalVo{vo.SignalBuy, vo.SignalSell, vo.SignalHold} {
		t.Run(string(signal), func(t *testing.T) {
			assert.Equal(t, signal, domains.NewSignalDomain(signalResultOf(signal)).Value())
		})
	}
}

func TestSignalDomainWantedDirection(t *testing.T) {
	testCases := []struct {
		name                string
		signal              vo.SignalVo
		expectedDirection   vo.PositionDirectionVo
		expectsAnyDirection bool
	}{
		{
			name:                "buy wants a long position",
			signal:              vo.SignalBuy,
			expectedDirection:   vo.PositionDirectionLong,
			expectsAnyDirection: true,
		},
		{
			name:                "sell wants a short position",
			signal:              vo.SignalSell,
			expectedDirection:   vo.PositionDirectionShort,
			expectsAnyDirection: true,
		},
		{name: "hold wants nothing", signal: vo.SignalHold, expectsAnyDirection: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			wantedDirection, wantsPosition :=
				domains.NewSignalDomain(signalResultOf(testCase.signal)).WantedDirection()

			assert.Equal(t, testCase.expectsAnyDirection, wantsPosition)
			if testCase.expectsAnyDirection {
				assert.Equal(t, testCase.expectedDirection, wantedDirection)
			}
		})
	}
}

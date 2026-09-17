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

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

// The three words a signal answers to, and what an unrecognised one comes out as.
//
// They live on the signal rather than on the message that prints them because two
// readers need them now: the message's source lines, and the trading mode working out
// what act to name in a conclusion. A copy in each is a copy that can drift.
func TestSignalDomainInWords(t *testing.T) {
	testCases := []struct {
		signal       vo.SignalVo
		expectedWord string
	}{
		{signal: vo.SignalBuy, expectedWord: "買入"},
		{signal: vo.SignalSell, expectedWord: "賣出"},
		{signal: vo.SignalHold, expectedWord: "持有"},
		// Written out as it stands rather than replaced with a guess: this is the
		// last place to quietly turn something the system did not understand into
		// one of the three things it did.
		{signal: vo.SignalVo("shrug"), expectedWord: "shrug"},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.signal), func(t *testing.T) {
			assert.Equal(t, testCase.expectedWord,
				domains.NewSignalDomainOf(testCase.signal).InWords())
		})
	}
}

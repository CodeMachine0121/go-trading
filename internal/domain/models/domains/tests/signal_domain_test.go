package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

// signalResultOf packs a signal under the well-known key, as the runner does.
func signalResultOf(signal vo.SignalVo) map[string]vo.IndicatorValueVo {
	return map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: signal}}
}

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

func TestSignalDomainInWords(t *testing.T) {
	testCases := []struct {
		signal       vo.SignalVo
		expectedWord string
	}{
		{signal: vo.SignalBuy, expectedWord: "買入"},
		{signal: vo.SignalSell, expectedWord: "賣出"},
		{signal: vo.SignalHold, expectedWord: "持有"},
		// Unrecognised values are shown verbatim, not guessed.
		{signal: vo.SignalVo("shrug"), expectedWord: "shrug"},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.signal), func(t *testing.T) {
			assert.Equal(t, testCase.expectedWord,
				domains.NewSignalDomainOf(testCase.signal).InWords())
		})
	}
}

// Cash and no opinion are distinct targets; reading a sell as "leave it" would carry a position the signal asked to exit.
func TestSignalDomainTargetPosition(t *testing.T) {
	testCases := []struct {
		signal         vo.SignalVo
		expectedTarget vo.TargetPositionVo
	}{
		{signal: vo.SignalBuy, expectedTarget: vo.TargetPositionLong},
		{signal: vo.SignalSell, expectedTarget: vo.TargetPositionFlat},
		{signal: vo.SignalHold, expectedTarget: vo.TargetPositionUnchanged},
		// Unrecognised signals leave the position unchanged rather than quietly closing it.
		{signal: vo.SignalVo("shrug"), expectedTarget: vo.TargetPositionUnchanged},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.signal), func(t *testing.T) {
			assert.Equal(t, testCase.expectedTarget,
				domains.NewSignalDomainOf(testCase.signal).TargetPosition())
		})
	}
}

// Only sell is translated, because the reader is usually flat and can't act on 賣出.
func TestSignalDomainHeadlineVerb(t *testing.T) {
	testCases := []struct {
		signal       vo.SignalVo
		expectedVerb string
	}{
		{signal: vo.SignalBuy, expectedVerb: "買入"},
		{signal: vo.SignalSell, expectedVerb: "出場"},
		{signal: vo.SignalHold, expectedVerb: "持有"},
		{signal: vo.SignalVo("shrug"), expectedVerb: "shrug"},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.signal), func(t *testing.T) {
			assert.Equal(t, testCase.expectedVerb,
				domains.NewSignalDomainOf(testCase.signal).HeadlineVerb())
		})
	}
}

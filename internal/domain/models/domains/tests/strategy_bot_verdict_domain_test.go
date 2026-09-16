package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewStrategyBotVerdictDomainReadsTwoConditionsAsOneConclusion(t *testing.T) {
	testCases := []struct {
		name              string
		buyConditionHolds bool
		sellConditionHold bool
		expectedVerdict   vo.StrategyBotVerdictVo
		expectedConflict  bool
	}{
		{
			name:              "only the buy condition holds",
			buyConditionHolds: true,
			expectedVerdict:   vo.StrategyBotVerdictBuy,
		},
		{
			name:              "only the sell condition holds",
			sellConditionHold: true,
			expectedVerdict:   vo.StrategyBotVerdictSell,
		},
		{
			name:            "neither holds",
			expectedVerdict: vo.StrategyBotVerdictNone,
		},
		{
			// Both at once is the bot saying it should buy and sell in the same
			// breath. No side is picked, because a side picked here is an opinion
			// its owner would act on and could never trace back.
			name:              "both hold at once",
			buyConditionHolds: true,
			sellConditionHold: true,
			expectedVerdict:   vo.StrategyBotVerdictConflict,
			expectedConflict:  true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verdict := domains.NewStrategyBotVerdictDomain(
				testCase.buyConditionHolds, testCase.sellConditionHold, "")

			assert.Equal(t, testCase.expectedVerdict, verdict.Verdict())
			assert.Equal(t, testCase.expectedConflict, verdict.IsConflicting())
		})
	}
}

func TestStrategyBotVerdictShouldSend(t *testing.T) {
	testCases := []struct {
		name              string
		buyConditionHolds bool
		sellConditionHold bool
		lastSentSignal    string
		expectedToSend    bool
		expectedSignal    vo.SignalVo
		expectsAnySignal  bool
	}{
		{
			// Nothing sent yet is what starting a bot leaves behind, so the first
			// conclusion always goes out: somebody who pressed play and then heard
			// nothing cannot tell a quiet market from a broken bot.
			name:              "the first conclusion after a start always goes out",
			buyConditionHolds: true,
			lastSentSignal:    "",
			expectedToSend:    true,
			expectedSignal:    vo.SignalBuy,
			expectsAnySignal:  true,
		},
		{
			name:              "the same conclusion again says nothing",
			buyConditionHolds: true,
			lastSentSignal:    string(vo.SignalBuy),
			expectedToSend:    false,
			expectedSignal:    vo.SignalBuy,
			expectsAnySignal:  true,
		},
		{
			name:              "a changed conclusion goes out",
			sellConditionHold: true,
			lastSentSignal:    string(vo.SignalBuy),
			expectedToSend:    true,
			expectedSignal:    vo.SignalSell,
			expectsAnySignal:  true,
		},
		{
			name:             "a quiet round says nothing and is not a signal",
			lastSentSignal:   string(vo.SignalBuy),
			expectedToSend:   false,
			expectsAnySignal: false,
		},
		{
			name:              "a conflicting round says nothing and is not a signal",
			buyConditionHolds: true,
			sellConditionHold: true,
			lastSentSignal:    string(vo.SignalBuy),
			expectedToSend:    false,
			expectsAnySignal:  false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verdict := domains.NewStrategyBotVerdictDomain(
				testCase.buyConditionHolds, testCase.sellConditionHold, testCase.lastSentSignal)

			assert.Equal(t, testCase.expectedToSend, verdict.ShouldSend())

			signal, hasSignal := verdict.Signal()
			assert.Equal(t, testCase.expectsAnySignal, hasSignal)
			if testCase.expectsAnySignal {
				assert.Equal(t, testCase.expectedSignal, signal)
			}
		})
	}
}

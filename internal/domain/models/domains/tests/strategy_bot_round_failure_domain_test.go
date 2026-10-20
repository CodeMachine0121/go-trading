package domains_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestNewStrategyBotRoundFailureDomainReadsWhatAFailedRoundMeans(t *testing.T) {
	testCases := []struct {
		name               string
		roundError         error
		expectedKind       string
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{
			name:               "a strategy script that was deleted stops the bot",
			roundError:         domains.StrategyScriptNotFound(7),
			expectedKind:       "halted",
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			// Withdrawn and deleted scripts halt identically so the halt reason does not
			// reveal whether another user's script exists.
			name:               "a strategy script withdrawn from the marketplace stops it the same way",
			roundError:         fmt.Errorf("%w", domains.ErrStrategyScriptNotPublished),
			expectedKind:       "halted",
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			name:               "a strategy script that is not the bot owner's stops it the same way",
			roundError:         domains.StrategyScriptNotYours(7),
			expectedKind:       "halted",
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			name:               "a script that will not run stops watching that symbol",
			roundError:         fmt.Errorf("%w: boom", domains.ErrIndicatorScriptFailed),
			expectedKind:       "watchedSymbolHalted",
			expectedHaltReason: vo.StrategyBotHaltScriptFailed,
		},
		{
			name:               "a script reaching for a knob nobody declared stops watching that symbol",
			roundError:         domains.UndeclaredParameter("週期"),
			expectedKind:       "watchedSymbolHalted",
			expectedHaltReason: vo.StrategyBotHaltScriptFailed,
		},
		{
			// Closed markets reopen, so halting would switch off every bot each weekend.
			name:         "a market that is not trading only skips the round",
			roundError:   domains.ObservationWindowHoldsNoTrading(vo.MarketVo("taiwanStock")),
			expectedKind: "skipped",
		},
		{
			name:         "candles that have not arrived yet only skip the round",
			roundError:   domains.CandleCoverageTooThin(10, 60),
			expectedKind: "skipped",
		},
		{
			// Waiting for a compartment is nobody's fault, so the next round simply tries again.
			name:         "every script compartment staying busy only skips the round",
			roundError:   fmt.Errorf("%w: every slot stayed taken", domains.ErrIndicatorScriptCompartmentsBusy),
			expectedKind: "skipped",
		},
		{
			// The same stretch is refused again every round, so waiting achieves nothing.
			name:               "order flow reaching back more days than one run may hold stops watching that symbol",
			roundError:         fmt.Errorf("%w: 訂單流一次至多涵蓋 31 天", domains.ErrContractOrderFlowRefused),
			expectedKind:       "watchedSymbolHalted",
			expectedHaltReason: vo.StrategyBotHaltOrderFlowRefused,
		},
		{
			// A step too fine for the symbol would otherwise skip every round without the owner hearing why.
			name:               "a bar holding too many price levels stops watching that symbol too",
			roundError:         fmt.Errorf("%w: 一格至多 1000 個價位", domains.ErrContractOrderFlowRefused),
			expectedKind:       "watchedSymbolHalted",
			expectedHaltReason: vo.StrategyBotHaltOrderFlowRefused,
		},
		{
			name:         "order flow not gathered in time only skips the round",
			roundError:   fmt.Errorf("%w：40 秒內沒能備齊", domains.ErrContractOrderFlowNotReady),
			expectedKind: "skipped",
		},
		{
			name:         "order flow the venue would not give only skips the round",
			roundError:   fmt.Errorf("%w：archive answered 503", domains.ErrContractOrderFlowUnavailable),
			expectedKind: "skipped",
		},
		{
			// Unknown failures only skip the round, since halting is the destructive answer.
			name:         "a failure nobody recognises only skips the round",
			roundError:   errors.New("the database went away"),
			expectedKind: "skipped",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := domains.NewStrategyBotRoundFailureDomain(testCase.roundError)

			outcome := failure.ToOutcomeDto("ETHUSDT")
			assert.Equal(t, "ETHUSDT", outcome.Symbol)
			assert.Equal(t, testCase.expectedKind, outcome.Kind)
			assert.Equal(t, string(testCase.expectedHaltReason), outcome.HaltReason)
			assert.Equal(t, testCase.expectedHaltReason, failure.HaltReason())
		})
	}
}

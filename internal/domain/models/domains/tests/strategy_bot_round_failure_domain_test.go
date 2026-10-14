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
		expectedToHalt     bool
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{
			name:               "a strategy script that was deleted stops the bot",
			roundError:         domains.StrategyScriptNotFound(7),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			// Withdrawn and deleted scripts halt identically so the halt reason does not
			// reveal whether another user's script exists.
			name:               "a strategy script withdrawn from the marketplace stops it the same way",
			roundError:         fmt.Errorf("%w", domains.ErrStrategyScriptNotPublished),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			name:               "a strategy script that is not the bot owner's stops it the same way",
			roundError:         domains.StrategyScriptNotYours(7),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			name:               "a script that will not run stops the bot",
			roundError:         fmt.Errorf("%w: boom", domains.ErrIndicatorScriptFailed),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltScriptFailed,
		},
		{
			name:               "a script reaching for a knob nobody declared stops the bot",
			roundError:         domains.UndeclaredParameter("週期"),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltScriptFailed,
		},
		{
			// Closed markets reopen, so halting would switch off every bot each weekend.
			name:           "a market that is not trading only skips the round",
			roundError:     domains.ObservationWindowHoldsNoTrading(vo.MarketVo("taiwanStock")),
			expectedToHalt: false,
		},
		{
			name:           "candles that have not arrived yet only skip the round",
			roundError:     domains.CandleCoverageTooThin(10, 60),
			expectedToHalt: false,
		},
		{
			// Waiting for a compartment is nobody's fault, so the next round simply tries again.
			name:           "every script compartment staying busy only skips the round",
			roundError:     fmt.Errorf("%w: every slot stayed taken", domains.ErrIndicatorScriptCompartmentsBusy),
			expectedToHalt: false,
		},
		{
			// The same stretch is refused again every round, so waiting achieves nothing.
			name:               "order flow reaching back more days than one run may hold stops the bot",
			roundError:         fmt.Errorf("%w: 訂單流一次至多涵蓋 31 天", domains.ErrContractOrderFlowRefused),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltOrderFlowRefused,
		},
		{
			// A step too fine for the symbol would otherwise skip every round without the owner hearing why.
			name:               "a bar holding too many price levels stops the bot too",
			roundError:         fmt.Errorf("%w: 一格至多 1000 個價位", domains.ErrContractOrderFlowRefused),
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltOrderFlowRefused,
		},
		{
			name:           "order flow not gathered in time only skips the round",
			roundError:     fmt.Errorf("%w：40 秒內沒能備齊", domains.ErrContractOrderFlowNotReady),
			expectedToHalt: false,
		},
		{
			name:           "order flow the venue would not give only skips the round",
			roundError:     fmt.Errorf("%w：archive answered 503", domains.ErrContractOrderFlowUnavailable),
			expectedToHalt: false,
		},
		{
			// Unknown failures only skip the round, since halting is the destructive answer.
			name:           "a failure nobody recognises only skips the round",
			roundError:     errors.New("the database went away"),
			expectedToHalt: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := domains.NewStrategyBotRoundFailureDomain(testCase.roundError)

			assert.Equal(t, testCase.expectedToHalt, failure.HaltsTheBot())
			assert.Equal(t, testCase.expectedHaltReason, failure.HaltReason())
		})
	}
}

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

func TestNewStrategyBotDeliveryFailureDomainSeparatesRetypingFromWaiting(t *testing.T) {
	testCases := []struct {
		name               string
		failureReason      vo.DeliveryFailureReasonVo
		expectedToHalt     bool
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{
			name:               "a rejected token needs a new token, so the bot stops",
			failureReason:      vo.DeliveryFailureCredentialRejected,
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltCredentialRejected,
		},
		{
			name:               "an unknown chat needs a new chat identifier, so the bot stops",
			failureReason:      vo.DeliveryFailureDestinationNotFound,
			expectedToHalt:     true,
			expectedHaltReason: vo.StrategyBotHaltDestinationNotFound,
		},
		{
			name:           "an unreachable Telegram needs nothing but time",
			failureReason:  vo.DeliveryFailureUnreachable,
			expectedToHalt: false,
		},
		{
			name:           "a Telegram that answered too late needs nothing but time",
			failureReason:  vo.DeliveryFailureTimedOut,
			expectedToHalt: false,
		},
		{
			name:           "a message that arrived is not a failure at all",
			failureReason:  vo.DeliveryFailureNone,
			expectedToHalt: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			failure := domains.NewStrategyBotDeliveryFailureDomain(testCase.failureReason)

			assert.Equal(t, testCase.expectedToHalt, failure.HaltsTheBot())
			assert.Equal(t, testCase.expectedHaltReason, failure.HaltReason())
		})
	}
}

func TestNewStrategyBotRoundFailureDomainHaltsWhenThereIsNowhereToSpeak(t *testing.T) {
	// 與其他三種等一等就好的失敗不同：沒有設定不會自己出現。
	failure := domains.NewStrategyBotRoundFailureDomain(domains.ErrTelegramDeliveryNotConfigured)

	assert.True(t, failure.HaltsTheBot())
	assert.Equal(t, vo.StrategyBotHaltDeliveryNotConfigured, failure.HaltReason())
}

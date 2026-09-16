package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aStoredBot is a bot as it comes back out of the store, so that each test changes
// the one field it is about.
func aStoredBot(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{
		ID:                     3,
		OwnerID:                7,
		Name:                   "早盤突破",
		Symbol:                 "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(runState),
	}
}

func TestStrategyBotRunStateStartPutsTheBotToWorkImmediatelyAndForgetsWhatItSaid(t *testing.T) {
	storedBot := aStoredBot(vo.StrategyBotStopped)
	storedBot.LastSentSignal = string(vo.SignalBuy)
	storedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)
	storedBot.Conflicting = true

	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	startedBot := domains.NewStrategyBotRunStateDomain(storedBot).Start(now)

	assert.Equal(t, string(vo.StrategyBotRunning), startedBot.RunState)
	// Due now, not in five minutes: a bot set to sixty would otherwise do nothing
	// for an hour after somebody pressed play.
	assert.Equal(t, now, startedBot.NextRunAt)
	// Starting over is starting over, which is what makes the first conclusion after
	// a start always go out.
	assert.Empty(t, startedBot.LastSentSignal)
	assert.Empty(t, startedBot.HaltReason)
	assert.False(t, startedBot.Conflicting)
}

func TestStrategyBotRunStateStopAndHaltDifferOnlyInWhetherAnybodyHasToFixSomething(t *testing.T) {
	storedBot := aStoredBot(vo.StrategyBotRunning)
	storedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)

	stoppedBot := domains.NewStrategyBotRunStateDomain(storedBot).Stop()
	assert.Equal(t, string(vo.StrategyBotStopped), stoppedBot.RunState)
	assert.Empty(t, stoppedBot.HaltReason)

	haltedBot := domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotRunning)).Halt(
		vo.StrategyBotHaltCredentialRejected)
	assert.Equal(t, string(vo.StrategyBotStopped), haltedBot.RunState)
	assert.Equal(t, string(vo.StrategyBotHaltCredentialRejected), haltedBot.HaltReason)
}

func TestStrategyBotRunStateRequireEditable(t *testing.T) {
	assert.NoError(t,
		domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotStopped)).RequireEditable())

	editableError := domains.NewStrategyBotRunStateDomain(
		aStoredBot(vo.StrategyBotRunning)).RequireEditable()

	require.ErrorIs(t, editableError, domains.ErrStrategyBotRunning)
	assert.ErrorContains(t, editableError, "要先停止它才改得動")
}

func TestStrategyBotRunStateRequireStartable(t *testing.T) {
	testCases := []struct {
		name               string
		hasDeliverySetting bool
		runningBotCount    int
		expectedSentinel   error
		expectedMessage    string
	}{
		{
			name:               "somewhere to speak and room to spare",
			hasDeliverySetting: true,
			runningBotCount:    9,
		},
		{
			// A bot that cannot send is a bot for which running and stopped are the
			// same state.
			name:               "nowhere to be spoken to",
			hasDeliverySetting: false,
			runningBotCount:    0,
			expectedSentinel:   domains.ErrStrategyBotDeliveryNotConfigured,
			expectedMessage:    "要先完成 Telegram 設定",
		},
		{
			name:               "already running as many as allowed",
			hasDeliverySetting: true,
			runningBotCount:    10,
			expectedSentinel:   domains.ErrStrategyBotRunningLimitReached,
			expectedMessage:    "上限是 10 台",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			startableError := domains.NewStrategyBotRunStateDomain(
				aStoredBot(vo.StrategyBotStopped)).RequireStartable(
				testCase.hasDeliverySetting, testCase.runningBotCount)

			if testCase.expectedSentinel == nil {
				assert.NoError(t, startableError)
				return
			}

			require.ErrorIs(t, startableError, testCase.expectedSentinel)
			assert.ErrorContains(t, startableError, testCase.expectedMessage)
		})
	}
}

func TestStrategyBotRunStateRoundFinished(t *testing.T) {
	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	testCases := []struct {
		name                   string
		lastSentSignal         string
		sentSignal             vo.SignalVo
		conflicting            bool
		expectedLastSentSignal string
	}{
		{
			name:                   "a message that arrived becomes the new last signal",
			lastSentSignal:         "",
			sentSignal:             vo.SignalBuy,
			expectedLastSentSignal: string(vo.SignalBuy),
		},
		{
			// A conclusion nobody received has not been said, so the next round
			// offers it again rather than assuming it got through.
			name:                   "a round that sent nothing leaves the last signal alone",
			lastSentSignal:         string(vo.SignalBuy),
			sentSignal:             "",
			expectedLastSentSignal: string(vo.SignalBuy),
		},
		{
			name:                   "a conflicting round leaves the last signal alone and marks the bot",
			lastSentSignal:         string(vo.SignalBuy),
			sentSignal:             "",
			conflicting:            true,
			expectedLastSentSignal: string(vo.SignalBuy),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			storedBot := aStoredBot(vo.StrategyBotRunning)
			storedBot.LastSentSignal = testCase.lastSentSignal

			finishedBot := domains.NewStrategyBotRunStateDomain(storedBot).RoundFinished(
				now, testCase.sentSignal, testCase.conflicting)

			assert.Equal(t, testCase.expectedLastSentSignal, finishedBot.LastSentSignal)
			assert.Equal(t, testCase.conflicting, finishedBot.Conflicting)
			// Measured from the round that happened, not from the one that was due,
			// which is the whole of "missed rounds are never made up".
			assert.Equal(t, now.Add(5*time.Minute), finishedBot.NextRunAt)
			assert.Equal(t, string(vo.StrategyBotRunning), finishedBot.RunState)
		})
	}
}

func TestStrategyBotRunStateRoundSkippedMovesTheClockAndNothingElse(t *testing.T) {
	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	storedBot := aStoredBot(vo.StrategyBotRunning)
	storedBot.LastSentSignal = string(vo.SignalBuy)
	storedBot.Conflicting = true

	skippedBot := domains.NewStrategyBotRunStateDomain(storedBot).RoundSkipped(now)

	assert.Equal(t, now.Add(5*time.Minute), skippedBot.NextRunAt)
	// A closed weekend must not read as a change of mind on Monday.
	assert.Equal(t, string(vo.SignalBuy), skippedBot.LastSentSignal)
	assert.True(t, skippedBot.Conflicting)
	assert.Equal(t, string(vo.StrategyBotRunning), skippedBot.RunState)
}

func TestStrategyBotRunStateIsRunning(t *testing.T) {
	assert.True(t,
		domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotRunning)).IsRunning())
	assert.False(t,
		domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotStopped)).IsRunning())
}

package domains_test

import (
	"slices"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aStoredBot(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{
		ID:                     3,
		OwnerID:                7,
		Name:                   "早盤突破",
		TriggerIntervalMinutes: 5,
		RunState:               string(runState),
		WatchedSymbols: []entities.StrategyBotWatchedSymbol{
			{StrategyBotID: 3, Symbol: "BTCUSDT"},
			{StrategyBotID: 3, Symbol: "ETHUSDT"},
		},
	}
}

func TestStrategyBotRunStateStartPutsTheBotToWorkImmediatelyAndForgetsWhatItSaid(t *testing.T) {
	roundDueAt := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	storedBot := aStoredBot(vo.StrategyBotStopped)
	storedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)
	storedBot.WatchedSymbols[0].LastSentSignal = string(vo.SignalBuy)
	storedBot.WatchedSymbols[0].LastSentRoundDueAt = &roundDueAt
	storedBot.WatchedSymbols[0].AutoOrderPositionDirection = string(vo.TargetPositionLong)
	storedBot.WatchedSymbols[0].AutoOrderPositionQuantity = decimal.RequireFromString("0.01")
	storedBot.WatchedSymbols[1].HaltReason = string(vo.StrategyBotHaltScriptFailed)
	storedBot.WatchedSymbols[1].Conflicting = true

	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)

	startedBot := domains.NewStrategyBotRunStateDomain(storedBot).Start(now)

	assert.Equal(t, string(vo.StrategyBotRunning), startedBot.RunState)
	// Due immediately so a long interval does not delay the first round after starting.
	assert.Equal(t, now, startedBot.NextRunAt)
	assert.Empty(t, startedBot.HaltReason)
	for _, watchedSymbol := range startedBot.WatchedSymbols {
		// Clearing the last signal makes the first conclusion after a start always go out.
		assert.Empty(t, watchedSymbol.LastSentSignal)
		assert.Nil(t, watchedSymbol.LastSentRoundDueAt)
		assert.False(t, watchedSymbol.Conflicting)
		assert.Empty(t, watchedSymbol.HaltReason)
	}
	// What auto orders opened is not forgotten by a restart.
	assert.Equal(t, string(vo.TargetPositionLong), startedBot.WatchedSymbols[0].AutoOrderPositionDirection)
	assert.True(t, decimal.RequireFromString("0.01").Equal(startedBot.WatchedSymbols[0].AutoOrderPositionQuantity))
	// The stored bot it was read from is left as it was.
	assert.Equal(t, string(vo.SignalBuy), storedBot.WatchedSymbols[0].LastSentSignal)
}

func TestStrategyBotRunStateStopAndHaltDifferOnlyInWhetherAnybodyHasToFixSomething(t *testing.T) {
	storedBot := aStoredBot(vo.StrategyBotRunning)
	storedBot.HaltReason = string(vo.StrategyBotHaltScriptFailed)

	stoppedBot := domains.NewStrategyBotRunStateDomain(storedBot).Stop()
	assert.Equal(t, string(vo.StrategyBotStopped), stoppedBot.RunState)
	assert.Empty(t, stoppedBot.HaltReason)

	haltedAt := time.Date(2026, 9, 16, 13, 2, 0, 0, time.UTC)
	haltedBot := domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotRunning)).Halt(
		vo.StrategyBotHaltCredentialRejected, haltedAt)
	assert.Equal(t, string(vo.StrategyBotStopped), haltedBot.RunState)
	assert.Equal(t, string(vo.StrategyBotHaltCredentialRejected), haltedBot.HaltReason)
	// Moved so a round still in flight for the bot can no longer be booked onto it.
	assert.Equal(t, haltedAt, haltedBot.NextRunAt)
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

func TestStrategyBotRunStateIsRunning(t *testing.T) {
	assert.True(t,
		domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotRunning)).IsRunning())
	assert.False(t,
		domains.NewStrategyBotRunStateDomain(aStoredBot(vo.StrategyBotStopped)).IsRunning())
}

func TestStrategyBotWatchedSymbolConcluded(t *testing.T) {
	roundDueAt := time.Date(2026, 9, 16, 12, 59, 0, 0, time.UTC)

	testCases := []struct {
		name                   string
		lastSentSignal         string
		sentSignal             vo.SignalVo
		conflicting            bool
		expectedLastSentSignal string
	}{
		{
			name:                   "a message that was queued becomes the new last signal",
			lastSentSignal:         "",
			sentSignal:             vo.SignalBuy,
			expectedLastSentSignal: string(vo.SignalBuy),
		},
		{
			// An unsent conclusion leaves the last signal alone so the next round offers it again.
			name:                   "a round that sent nothing leaves the last signal alone",
			lastSentSignal:         string(vo.SignalBuy),
			sentSignal:             "",
			expectedLastSentSignal: string(vo.SignalBuy),
		},
		{
			name:                   "a conflicting round leaves the last signal alone and marks the symbol",
			lastSentSignal:         string(vo.SignalBuy),
			sentSignal:             "",
			conflicting:            true,
			expectedLastSentSignal: string(vo.SignalBuy),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			concludedSymbol := domains.NewStrategyBotWatchedSymbolDomain(entities.StrategyBotWatchedSymbol{
				Symbol: "ETHUSDT", LastSentSignal: testCase.lastSentSignal,
			}).Concluded(roundDueAt, testCase.sentSignal, testCase.conflicting)

			assert.Equal(t, testCase.expectedLastSentSignal, concludedSymbol.LastSentSignal)
			// The round that said the signal is remembered with it, so only that round's message can make the symbol forget it.
			if testCase.sentSignal != "" {
				require.NotNil(t, concludedSymbol.LastSentRoundDueAt)
				assert.Equal(t, roundDueAt, *concludedSymbol.LastSentRoundDueAt)
			} else {
				assert.Nil(t, concludedSymbol.LastSentRoundDueAt)
			}
			assert.Equal(t, testCase.conflicting, concludedSymbol.Conflicting)
		})
	}
}

func TestStrategyBotWatchedSymbolRequireRemovable(t *testing.T) {
	testCases := []struct {
		name                  string
		positionDirection     vo.TargetPositionVo
		hasUnsettledAutoOrder bool
		expectedSentinel      error
		expectedMessage       string
	}{
		{name: "flat and with nothing in flight", positionDirection: ""},
		{name: "a position recorded as flat", positionDirection: vo.TargetPositionFlat},
		{
			name: "still holding what auto orders opened", positionDirection: vo.TargetPositionLong,
			expectedSentinel: domains.ErrStrategyBotWatchedSymbolHoldsPosition,
			expectedMessage:  "ETHUSDT 還有機器人持倉，要先讓它空手才能移除",
		},
		{
			name: "an auto order not yet done", positionDirection: "", hasUnsettledAutoOrder: true,
			expectedSentinel: domains.ErrStrategyBotWatchedSymbolAutoOrderOutstanding,
			expectedMessage:  "ETHUSDT 還有沒做完的自動下單，要等它做完才能移除",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			removableError := domains.NewStrategyBotWatchedSymbolDomain(entities.StrategyBotWatchedSymbol{
				Symbol: "ETHUSDT", AutoOrderPositionDirection: string(testCase.positionDirection),
			}).RequireRemovable(testCase.hasUnsettledAutoOrder)

			if testCase.expectedSentinel == nil {
				assert.NoError(t, removableError)
				return
			}

			require.ErrorIs(t, removableError, testCase.expectedSentinel)
			assert.ErrorContains(t, removableError, testCase.expectedMessage)
		})
	}
}

// aWatchedSymbolOutcome reads one symbol's outcome as the round's DTO would carry it.
func aWatchedSymbolOutcome(
	symbol string, kind string, haltReason vo.StrategyBotHaltReasonVo, verdict vo.StrategyBotVerdictVo,
	sentSignal vo.SignalVo, conflicting bool,
) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	return dto.StrategyBotWatchedSymbolRoundOutcomeDto{
		Symbol: symbol, Kind: kind, HaltReason: string(haltReason), Verdict: string(verdict),
		SentSignal: string(sentSignal), Conflicting: conflicting,
	}
}

func TestStrategyBotRoundOutcomeApplyTo(t *testing.T) {
	now := time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)
	dueAt := now.Add(-time.Minute)

	testCases := []struct {
		name                     string
		alreadyHaltedSymbols     []string
		watchedSymbolOutcomes    []dto.StrategyBotWatchedSymbolRoundOutcomeDto
		expectedRunState         vo.StrategyBotRunStateVo
		expectedBotHaltReason    vo.StrategyBotHaltReasonVo
		expectedLastSentSignals  []string
		expectedConflicting      []bool
		expectedSymbolHalts      []string
		expectsTheClockScheduled bool
	}{
		{
			name: "each symbol keeps its own conclusion",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictConflict, "", true),
				aWatchedSymbolOutcome("ETHUSDT", "concluded", "", vo.StrategyBotVerdictSell, vo.SignalSell, false),
			},
			expectedRunState:         vo.StrategyBotRunning,
			expectedLastSentSignals:  []string{string(vo.SignalBuy), string(vo.SignalSell)},
			expectedConflicting:      []bool{true, false},
			expectedSymbolHalts:      []string{"", ""},
			expectsTheClockScheduled: true,
		},
		{
			name: "a skipped symbol moves nothing of its own",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "skipped", "", "", "", false),
				aWatchedSymbolOutcome("ETHUSDT", "concluded", "", vo.StrategyBotVerdictBuy, "", false),
			},
			expectedRunState:         vo.StrategyBotRunning,
			expectedLastSentSignals:  []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:      []bool{true, false},
			expectedSymbolHalts:      []string{"", ""},
			expectsTheClockScheduled: true,
		},
		{
			name: "a symbol halting on its own leaves the bot running",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictBuy, "", false),
				aWatchedSymbolOutcome("ETHUSDT", "watchedSymbolHalted", vo.StrategyBotHaltScriptFailed, "", "", false),
			},
			expectedRunState:         vo.StrategyBotRunning,
			expectedLastSentSignals:  []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:      []bool{false, true},
			expectedSymbolHalts:      []string{"", string(vo.StrategyBotHaltScriptFailed)},
			expectsTheClockScheduled: true,
		},
		{
			name:                 "the last symbol halting stops the bot for its reason",
			alreadyHaltedSymbols: []string{"ETHUSDT"},
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "watchedSymbolHalted", vo.StrategyBotHaltOrderFlowRefused, "", "", false),
			},
			expectedRunState:        vo.StrategyBotStopped,
			expectedBotHaltReason:   vo.StrategyBotHaltOrderFlowRefused,
			expectedLastSentSignals: []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:     []bool{true, true},
			expectedSymbolHalts:     []string{string(vo.StrategyBotHaltOrderFlowRefused), string(vo.StrategyBotHaltScriptFailed)},
		},
		{
			name: "every symbol halting in one round stops the bot for the last one's reason",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "watchedSymbolHalted", vo.StrategyBotHaltOrderFlowRefused, "", "", false),
				aWatchedSymbolOutcome("ETHUSDT", "watchedSymbolHalted", vo.StrategyBotHaltScriptFailed, "", "", false),
			},
			expectedRunState:        vo.StrategyBotStopped,
			expectedBotHaltReason:   vo.StrategyBotHaltScriptFailed,
			expectedLastSentSignals: []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:     []bool{true, true},
			expectedSymbolHalts:     []string{string(vo.StrategyBotHaltOrderFlowRefused), string(vo.StrategyBotHaltScriptFailed)},
		},
		{
			name: "a failure every symbol meets stops the whole bot",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "halted", vo.StrategyBotHaltTradingStrategyUnavailable, "", "", false),
				aWatchedSymbolOutcome("ETHUSDT", "halted", vo.StrategyBotHaltTradingStrategyUnavailable, "", "", false),
			},
			expectedRunState:        vo.StrategyBotStopped,
			expectedBotHaltReason:   vo.StrategyBotHaltTradingStrategyUnavailable,
			expectedLastSentSignals: []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:     []bool{true, true},
			expectedSymbolHalts:     []string{"", ""},
		},
		{
			name: "a halt without a reason reads as a skip",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("BTCUSDT", "halted", "", "", "", false),
				aWatchedSymbolOutcome("ETHUSDT", "watchedSymbolHalted", "", "", "", false),
			},
			expectedRunState:         vo.StrategyBotRunning,
			expectedLastSentSignals:  []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:      []bool{true, true},
			expectedSymbolHalts:      []string{"", ""},
			expectsTheClockScheduled: true,
		},
		{
			name: "a symbol dropped while the round ran changes nothing",
			watchedSymbolOutcomes: []dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				aWatchedSymbolOutcome("SOLUSDT", "watchedSymbolHalted", vo.StrategyBotHaltScriptFailed, "", "", false),
			},
			expectedRunState:         vo.StrategyBotRunning,
			expectedLastSentSignals:  []string{string(vo.SignalBuy), string(vo.SignalBuy)},
			expectedConflicting:      []bool{true, true},
			expectedSymbolHalts:      []string{"", ""},
			expectsTheClockScheduled: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			storedBot := aStoredBot(vo.StrategyBotRunning)
			for index := range storedBot.WatchedSymbols {
				storedBot.WatchedSymbols[index].LastSentSignal = string(vo.SignalBuy)
				storedBot.WatchedSymbols[index].Conflicting = true
				if slices.Contains(testCase.alreadyHaltedSymbols, storedBot.WatchedSymbols[index].Symbol) {
					storedBot.WatchedSymbols[index].HaltReason = string(vo.StrategyBotHaltScriptFailed)
				}
			}

			endedBot, botHaltReason := domains.NewStrategyBotRoundOutcomeDomain(dto.StrategyBotRoundOutcomeDto{
				WatchedSymbolOutcomes: testCase.watchedSymbolOutcomes,
			}).ApplyTo(domains.NewStrategyBotRunStateDomain(storedBot), dueAt, now)

			assert.Equal(t, string(testCase.expectedRunState), endedBot.RunState)
			assert.Equal(t, testCase.expectedBotHaltReason, botHaltReason)
			assert.Equal(t, string(testCase.expectedBotHaltReason), endedBot.HaltReason)
			for index, watchedSymbol := range endedBot.WatchedSymbols {
				assert.Equal(t, testCase.expectedLastSentSignals[index], watchedSymbol.LastSentSignal, watchedSymbol.Symbol)
				assert.Equal(t, testCase.expectedConflicting[index], watchedSymbol.Conflicting, watchedSymbol.Symbol)
				assert.Equal(t, testCase.expectedSymbolHalts[index], watchedSymbol.HaltReason, watchedSymbol.Symbol)
			}
			if testCase.expectsTheClockScheduled {
				// Measured from the round that ran, so missed rounds are never made up.
				assert.Equal(t, now.Add(5*time.Minute), endedBot.NextRunAt)
			} else {
				// Moved so a round still in flight can no longer be booked onto the halted bot.
				assert.Equal(t, now, endedBot.NextRunAt)
			}
		})
	}
}

func TestStrategyBotWatchedSymbolRoundOutcomeRecordedResult(t *testing.T) {
	testCases := []struct {
		name           string
		outcome        dto.StrategyBotWatchedSymbolRoundOutcomeDto
		expectedResult vo.StrategyBotRoundResultVo
	}{
		{
			name:           "買入就記買入",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictBuy, vo.SignalBuy, false),
			expectedResult: vo.StrategyBotRoundResultBuy,
		},
		{
			// 記的是這一輪想的判斷，而非送出的訊號。
			name:           "想了買入但沒送出去，還是記買入",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictBuy, "", false),
			expectedResult: vo.StrategyBotRoundResultBuy,
		},
		{
			name:           "賣出就記賣出",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictSell, vo.SignalSell, false),
			expectedResult: vo.StrategyBotRoundResultSell,
		},
		{
			name:           "沒有結論記成持有",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictNone, "", false),
			expectedResult: vo.StrategyBotRoundResultHold,
		},
		{
			// 打架與持有分開記：持有在等市場，打架在等主人改條件。
			name:           "打架就記打架",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "concluded", "", vo.StrategyBotVerdictConflict, "", true),
			expectedResult: vo.StrategyBotRoundResultConflict,
		},
		{
			name:           "跳過記成持有",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "skipped", "", "", "", false),
			expectedResult: vo.StrategyBotRoundResultHold,
		},
		{
			name:           "標的停擺記成持有",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "watchedSymbolHalted", vo.StrategyBotHaltScriptFailed, "", "", false),
			expectedResult: vo.StrategyBotRoundResultHold,
		},
		{
			name:           "整台停擺記成持有",
			outcome:        aWatchedSymbolOutcome("BTCUSDT", "halted", vo.StrategyBotHaltTradingStrategyUnavailable, "", "", false),
			expectedResult: vo.StrategyBotRoundResultHold,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedResult,
				domains.NewStrategyBotWatchedSymbolRoundOutcomeDomain(testCase.outcome).RecordedResult())
		})
	}
}

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errBotStorageUnreachable = errors.New("bot storage unreachable")

func TestTradingStrategyApplicationInspectRewriteAnswersWithoutWriting(t *testing.T) {
	// Save is unstubbed: an inspection never writes.
	underTest := newTradingStrategyApplicationUnderTest(t)
	stored := storedTradingStrategy()
	stored.UpdatedAt = time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC)
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).Return(stored, nil).AnyTimes()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).Return(aScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()
	underTest.expectFollowingBots(aBotFollowingTheRules("夜班", vo.StrategyBotRunning))
	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID

	target, inspectError := underTest.tradingStrategyApplication.InspectTradingStrategyRewrite(
		t.Context(), strategyBotOwnerID, writeDto)

	// A running bot does not refuse an inspection; it is counted, and refused when the rewrite is carried out.
	require.NoError(t, inspectError)
	assert.Equal(t, tradingStrategyID, target.ID)
	assert.Equal(t, "黃金交叉", target.Name)
	assert.Equal(t, stored.UpdatedAt, target.UpdatedAt)
	assert.Equal(t, 1, target.BotReferenceCount)
}

func TestTradingStrategyApplicationInspectRewriteRefuses(t *testing.T) {
	testCases := []struct {
		name          string
		arrange       func(underTest tradingStrategyApplicationUnderTest)
		breakIt       func(writeName *string)
		expectedError error
	}{
		{
			name: "someone else's strategy, as missing",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				stored := storedTradingStrategy()
				stored.OwnerID = strategyBotOwnerID + 1
				underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).Return(stored, nil)
			},
			expectedError: domains.ErrTradingStrategyNotFound,
		},
		{
			name: "a source naming a script it cannot see",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
					Return(storedTradingStrategy(), nil).AnyTimes()
				underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
					Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(9))
			},
			expectedError: domains.ErrStrategyScriptNotFound,
		},
		{
			name: "content the rules refuse",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
					Return(storedTradingStrategy(), nil).AnyTimes()
				underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).Return(aScriptOwnedByTheCaller(9), nil)
				underTest.expectNoMarketplaceQuestion()
			},
			breakIt:       func(writeName *string) { *writeName = "" },
			expectedError: domains.ErrTradingStrategyValidation,
		},
		{
			name: "bots that cannot be read",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
					Return(storedTradingStrategy(), nil).AnyTimes()
				underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).Return(aScriptOwnedByTheCaller(9), nil)
				underTest.expectNoMarketplaceQuestion()
				underTest.strategyBotRepository.EXPECT().FindAllByTradingStrategy(gomock.Any(), tradingStrategyID).
					Return(nil, errBotStorageUnreachable)
			},
			expectedError: errBotStorageUnreachable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newTradingStrategyApplicationUnderTest(t)
			testCase.arrange(underTest)
			writeDto := aTradingStrategyWrite()
			writeDto.ID = tradingStrategyID
			if testCase.breakIt != nil {
				testCase.breakIt(&writeDto.Name)
			}

			_, inspectError := underTest.tradingStrategyApplication.InspectTradingStrategyRewrite(
				t.Context(), strategyBotOwnerID, writeDto)

			require.ErrorIs(t, inspectError, testCase.expectedError)
		})
	}
}

func TestStrategyScriptApplicationReportsBotsThatCannotBeRead(t *testing.T) {
	testCases := []struct {
		name string
		act  func(fixture strategyScriptApplicationUnderTest) error
	}{
		{
			name: "while rewriting",
			act: func(fixture strategyScriptApplicationUnderTest) error {
				writeDto := aStrategyScriptWrite()
				writeDto.ID = 7
				_, updateError := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

				return updateError
			},
		},
		{
			name: "while inspecting a rewrite",
			act: func(fixture strategyScriptApplicationUnderTest) error {
				writeDto := aStrategyScriptWrite()
				writeDto.ID = 7
				_, inspectError := fixture.strategyScriptApplication.InspectStrategyScriptRewrite(t.Context(), writeDto)

				return inspectError
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptApplicationUnderTest(t)
			fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(aStoredStrategyScript(7, "二十根均線"), nil).AnyTimes()
			*fixture.botLookupError = errBotStorageUnreachable

			assert.ErrorIs(t, testCase.act(fixture), errBotStorageUnreachable)
		})
	}
}

func TestStrategyScriptApplicationInspectRewriteCountsEveryBotUsingTheScript(t *testing.T) {
	// Update is unstubbed: an inspection never writes, and a running bot is counted rather than refused.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.expectStoredForRewrite(aStoredStrategyScript(7, "二十根均線"), []entities.StrategyBot{
		aBotUsingTheScript(strategyScriptOwnerID, "早盤突破", vo.StrategyBotRunning),
		aBotUsingTheScript(strategyScriptOwnerID+1, "別人的機器人", vo.StrategyBotStopped),
	})
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 7

	target, inspectError := fixture.strategyScriptApplication.InspectStrategyScriptRewrite(t.Context(), writeDto)

	require.NoError(t, inspectError)
	assert.Equal(t, uint(7), target.ID)
	assert.Equal(t, "二十根均線", target.Name)
	assert.Equal(t, time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC), target.UpdatedAt)
	assert.Equal(t, 2, target.BotReferenceCount)
}

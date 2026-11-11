package application_test

import (
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aStoredScriptUsedAs is the caller's script with a usage already decided.
func aStoredScriptUsedAs(id uint, usage string) entities.StrategyScript {
	strategyScript := aStoredStrategyScript(id, "均線")
	strategyScript.Usage = usage

	return strategyScript
}

// expectComputingWith answers what computes with the script.
func (fixture strategyScriptApplicationUnderTest) expectComputingWith(
	strategyScriptID uint, tradingStrategyNames []string, screeningProfileNames []string,
) {
	fixture.tradingStrategyRepository.EXPECT().
		FindNamesByStrategyScript(gomock.Any(), strategyScriptID).Return(tradingStrategyNames, nil).AnyTimes()
	fixture.screeningProfileRepository.EXPECT().
		FindNamesByStrategyScript(gomock.Any(), strategyScriptID).Return(screeningProfileNames, nil).AnyTimes()
}

func TestStrategyScriptApplicationSwitchesUsage(t *testing.T) {
	testCases := []struct {
		name          string
		stored        entities.StrategyScript
		requested     string
		expectedUsage string
	}{
		{name: "a calculation script nothing computes with becomes a chart script",
			stored: aStoredScriptUsedAs(4, "calculation"), requested: "chart", expectedUsage: "chart"},
		{name: "a chart script becomes a calculation script",
			stored: aStoredScriptUsedAs(4, "chart"), requested: "calculation", expectedUsage: "calculation"},
		{name: "a marketplace copy switches too",
			stored: func() entities.StrategyScript {
				copied := aMarketplaceCopy(4, "均線")
				copied.Usage = "calculation"
				return copied
			}(), requested: "chart", expectedUsage: "chart"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptApplicationUnderTest(t)
			fixture.expectComputingWith(4, []string{}, []string{})
			fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).Return(testCase.stored, nil)
			switched := testCase.stored
			switched.Usage = testCase.expectedUsage
			fixture.strategyScriptRepository.EXPECT().
				UpdateUsage(gomock.Any(), uint(4), testCase.expectedUsage).Return(switched, nil)

			strategyScriptDto, switchError := fixture.strategyScriptApplication.SwitchStrategyScriptUsage(
				t.Context(), strategyScriptOwnerID, 4, testCase.requested)

			require.NoError(t, switchError)
			assert.Equal(t, testCase.expectedUsage, strategyScriptDto.Usage)
			if testCase.stored.IsAdoptedFromMarketplace {
				assert.Empty(t, strategyScriptDto.Script, "a copy's algorithm stays its author's")
			}
		})
	}
}

func TestStrategyScriptApplicationKeepsASwitchedCopyUnrewritable(t *testing.T) {
	// No Update is stubbed: the copy's algorithm stays locked whatever its usage.
	fixture := newStrategyScriptApplicationUnderTest(t)
	copied := aMarketplaceCopy(4, "均線")
	copied.Usage = "chart"
	fixture.expectStoredForRewrite(copied, nil)
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 4

	_, rewriteError := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, rewriteError, domains.ErrStrategyScriptFromMarketplace)
}

func TestStrategyScriptApplicationKeepsAUsageSwitchedToItself(t *testing.T) {
	// No UpdateUsage is stubbed: switching to the usage it has writes nothing.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.expectComputingWith(4, []string{"4H 趨勢"}, []string{})
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).
		Return(aStoredScriptUsedAs(4, "calculation"), nil)

	strategyScriptDto, switchError := fixture.strategyScriptApplication.SwitchStrategyScriptUsage(
		t.Context(), strategyScriptOwnerID, 4, "calculation")

	require.NoError(t, switchError)
	assert.Equal(t, "calculation", strategyScriptDto.Usage)
}

func TestStrategyScriptApplicationRefusesAUsageSwitch(t *testing.T) {
	testCases := []struct {
		name                  string
		stored                entities.StrategyScript
		requested             string
		tradingStrategyNames  []string
		screeningProfileNames []string
		expectedError         error
		expectedMessage       string
	}{
		{name: "to the chart while a trading strategy computes with it",
			stored: aStoredScriptUsedAs(4, "calculation"), requested: "chart",
			tradingStrategyNames: []string{"4H 趨勢"}, screeningProfileNames: []string{},
			expectedError: domains.ErrStrategyScriptUsageInUse, expectedMessage: "交易策略「4H 趨勢」"},
		{name: "to the chart while a screening profile computes with it",
			stored: aStoredScriptUsedAs(4, "calculation"), requested: "chart",
			tradingStrategyNames: []string{}, screeningProfileNames: []string{"新幣"},
			expectedError: domains.ErrStrategyScriptUsageInUse, expectedMessage: "篩選設定「新幣」"},
		{name: "somebody else's script reads as missing",
			stored: func() entities.StrategyScript {
				strangers := aStoredScriptUsedAs(4, "calculation")
				strangers.OwnerID = strategyScriptOwnerID + 1
				return strangers
			}(), requested: "chart",
			tradingStrategyNames: []string{}, screeningProfileNames: []string{},
			expectedError: domains.ErrStrategyScriptNotFound, expectedMessage: "找不到識別碼為 4 的策略腳本"},
		{name: "a usage outside the two",
			stored: aStoredScriptUsedAs(4, "calculation"), requested: "backtest",
			tradingStrategyNames: []string{}, screeningProfileNames: []string{},
			expectedError: domains.ErrStrategyScriptValidation, expectedMessage: "用途只能是"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// No UpdateUsage is stubbed: every refusal lands before anything is written.
			fixture := newStrategyScriptApplicationUnderTest(t)
			fixture.expectComputingWith(4, testCase.tradingStrategyNames, testCase.screeningProfileNames)
			fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).Return(testCase.stored, nil)

			_, switchError := fixture.strategyScriptApplication.SwitchStrategyScriptUsage(
				t.Context(), strategyScriptOwnerID, 4, testCase.requested)

			require.ErrorIs(t, switchError, testCase.expectedError)
			assert.Contains(t, switchError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyScriptApplicationRefusesAUsageCarriedByARewrite(t *testing.T) {
	// No Update is stubbed: the refusal lands before anything is written.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.expectStoredForRewrite(aStoredScriptUsedAs(4, "calculation"), nil)
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 4
	writeDto.Usage = "chart"

	_, rewriteError := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, rewriteError, domains.ErrStrategyScriptUsageNotRewritable)
	assert.Contains(t, rewriteError.Error(), "請改用切換用途")
}

func TestStrategyScriptApplicationRewriteKeepsTheStoredUsage(t *testing.T) {
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.expectStoredForRewrite(aStoredScriptUsedAs(4, "chart"), nil)
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 4
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, rewritten entities.StrategyScript) (entities.StrategyScript, error) {
			return rewritten, nil
		})

	strategyScriptDto, rewriteError := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.NoError(t, rewriteError)
	assert.Equal(t, "chart", strategyScriptDto.Usage)
}

func TestStrategyScriptApplicationReportsAUsageSwitchItCouldNotCarryOut(t *testing.T) {
	storageFailure := errors.New("storage unreachable")
	testCases := []struct {
		name  string
		setUp func(fixture strategyScriptApplicationUnderTest)
	}{
		{name: "the trading strategies could not be read", setUp: func(fixture strategyScriptApplicationUnderTest) {
			fixture.tradingStrategyRepository.EXPECT().FindNamesByStrategyScript(gomock.Any(), uint(4)).Return(nil, storageFailure)
		}},
		{name: "the screening profiles could not be read", setUp: func(fixture strategyScriptApplicationUnderTest) {
			fixture.tradingStrategyRepository.EXPECT().FindNamesByStrategyScript(gomock.Any(), uint(4)).Return([]string{}, nil)
			fixture.screeningProfileRepository.EXPECT().FindNamesByStrategyScript(gomock.Any(), uint(4)).Return(nil, storageFailure)
		}},
		{name: "the usage could not be written", setUp: func(fixture strategyScriptApplicationUnderTest) {
			fixture.expectComputingWith(4, []string{}, []string{})
			fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).Return(aStoredScriptUsedAs(4, "calculation"), nil)
			fixture.strategyScriptRepository.EXPECT().UpdateUsage(gomock.Any(), uint(4), "chart").
				Return(entities.StrategyScript{}, storageFailure)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptApplicationUnderTest(t)
			testCase.setUp(fixture)

			_, switchError := fixture.strategyScriptApplication.SwitchStrategyScriptUsage(
				t.Context(), strategyScriptOwnerID, 4, "chart")

			require.ErrorIs(t, switchError, storageFailure)
		})
	}
}

func TestStrategyScriptApplicationRefusesASwitchThatSaysNoUsage(t *testing.T) {
	// No UpdateUsage is stubbed: a blank switch never reaches storage.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.expectComputingWith(4, []string{}, []string{})
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).Return(aStoredScriptUsedAs(4, "chart"), nil)

	_, switchError := fixture.strategyScriptApplication.SwitchStrategyScriptUsage(
		t.Context(), strategyScriptOwnerID, 4, "  ")

	require.ErrorIs(t, switchError, domains.ErrStrategyScriptValidation)
	assert.Contains(t, switchError.Error(), "切換用途時必須說要切成 chart 或 calculation")
}

package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aBotWriteDto passes every rule, so each test changes only the field under test.
func aBotWriteDto() dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		OwnerID:                7,
		Name:                   "早盤突破",
		Symbol:                 "BTCUSDT",
		TradingStrategyID:      9,
		TriggerIntervalMinutes: 5,
	}
}

func TestNewStrategyBotDomainAcceptsAWellFormedBot(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aBotWriteDto())
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()

	assert.Equal(t, uint(7), botEntity.OwnerID)
	assert.Equal(t, "早盤突破", botEntity.Name)
	assert.Equal(t, "BTCUSDT", botEntity.Symbol)
	assert.Equal(t, uint(9), botEntity.TradingStrategyID)
	assert.Equal(t, 5, botEntity.TriggerIntervalMinutes)
	assert.Equal(t, string(vo.StrategyBotStopped), botEntity.RunState)
}

func TestNewStrategyBotDomainKeepsNeitherBlanksNorAnIdentifierItWasNotGiven(t *testing.T) {
	writeDto := aBotWriteDto()
	writeDto.Name = "  早盤突破  "
	writeDto.Symbol = "  BTCUSDT  "

	strategyBot, buildError := domains.NewStrategyBotDomain(writeDto)
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()
	assert.Equal(t, "早盤突破", botEntity.Name)
	assert.Equal(t, "BTCUSDT", botEntity.Symbol)
}

func TestNewStrategyBotDomainRefusals(t *testing.T) {
	testCases := []struct {
		name            string
		mutate          func(writeDto *dto.StrategyBotWriteDto)
		expectedMessage string
	}{
		{
			name:            "a bot with nobody behind it",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.OwnerID = 0 },
			expectedMessage: "必須屬於一位使用者",
		},
		{
			name:            "a name of only blanks",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.Name = "   " },
			expectedMessage: "取一個名稱",
		},
		{
			name: "a name one character over the limit",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.Name = strings.Repeat("名", 129)
			},
			expectedMessage: "長度上限為 128 個字",
		},
		{
			// PostgreSQL cannot store NUL in text, so it is refused as a bad name rather
			// than failing at storage.
			name:            "a name carrying a null character",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.Name = "早盤\x00突破" },
			expectedMessage: "不得包含空字元",
		},
		{
			name:            "no symbol to watch",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.Symbol = "  " },
			expectedMessage: "要盯哪一個交易標的",
		},
		{
			name:            "a trigger interval of nothing",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.TriggerIntervalMinutes = 0 },
			expectedMessage: "觸發間隔必須大於零",
		},
		{
			name:            "a trigger interval one minute over a day",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.TriggerIntervalMinutes = 1441 },
			expectedMessage: "觸發間隔上限是 1440 分鐘",
		},
		{
			name:            "no trading strategy named",
			mutate:          func(writeDto *dto.StrategyBotWriteDto) { writeDto.TradingStrategyID = 0 },
			expectedMessage: "必須指名這台機器人要用哪一份交易策略",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aBotWriteDto()
			testCase.mutate(&writeDto)

			_, buildError := domains.NewStrategyBotDomain(writeDto)

			require.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

// A bot only names a trading strategy, so several bots can share one set of rules.
func TestNewStrategyBotDomainCarriesNoRulesOfItsOwn(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aBotWriteDto())
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()

	assert.Equal(t, uint(9), botEntity.TradingStrategyID)
	assert.Zero(t, botEntity.TradingStrategy.ID)
}

// Lower-case symbols are normalised rather than refused so writes and later reads agree.
func TestNewStrategyBotDomainStoresTheSymbolInOneCase(t *testing.T) {
	testCases := []struct {
		name           string
		writtenSymbol  string
		expectedSymbol string
	}{
		{name: "已經是大寫的原樣存起來", writtenSymbol: "BTCUSDT", expectedSymbol: "BTCUSDT"},
		{name: "小寫的存成大寫", writtenSymbol: "btcusdt", expectedSymbol: "BTCUSDT"},
		{name: "大小寫混著的也存成大寫", writtenSymbol: "BtcUsdt", expectedSymbol: "BTCUSDT"},
		{name: "前後的空白照樣去掉", writtenSymbol: "  ethusdt  ", expectedSymbol: "ETHUSDT"},
		{name: "沒有大小寫之分的台股代號原樣存起來", writtenSymbol: "0050", expectedSymbol: "0050"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aBotWriteDto()
			writeDto.Symbol = testCase.writtenSymbol

			strategyBot, buildError := domains.NewStrategyBotDomain(writeDto)
			require.NoError(t, buildError)

			assert.Equal(t, testCase.expectedSymbol, strategyBot.ToEntity().Symbol)
		})
	}
}

// aPositionPlannedBotWriteDto stakes a tenth of 50000 with 3% and 5% exits.
func aPositionPlannedBotWriteDto() dto.StrategyBotWriteDto {
	writeDto := aBotWriteDto()
	writeDto.PositionPlan = dto.PositionPlanSettingsDto{
		Capital:              decimal.NewFromInt(50000),
		SizingMode:           string(vo.PositionSizingModePercentage),
		SizingValue:          decimal.NewFromInt(10),
		StopLossPercentage:   decimal.NewFromInt(3),
		TakeProfitPercentage: decimal.NewFromInt(5),
	}

	return writeDto
}

func TestNewStrategyBotDomainKeepsThePositionPlanItWasGiven(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aPositionPlannedBotWriteDto())
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()

	assert.Equal(t, "50000", botEntity.PositionPlanCapital.String())
	assert.Equal(t,
		string(vo.PositionSizingModePercentage), botEntity.PositionPlanSizingMode)
	assert.Equal(t, "10", botEntity.PositionPlanSizingValue.String())
	assert.Equal(t, "3", botEntity.PositionPlanStopLossPercentage.String())
	assert.Equal(t, "5", botEntity.PositionPlanTakeProfitPercentage.String())
}

// Bots stored before position plans existed read as having none.
func TestNewStrategyBotDomainAcceptsABotWithNoPositionPlan(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aBotWriteDto())
	require.NoError(t, buildError)

	assert.True(t, strategyBot.ToEntity().PositionPlanCapital.IsZero())
}

func TestNewStrategyBotDomainRefusesAPositionPlanItCannotUse(t *testing.T) {
	testCases := []struct {
		name          string
		adjust        func(settings *dto.PositionPlanSettingsDto)
		expectedWords string
	}{
		{
			// The replay's own wording is reused so bots and replays cannot disagree.
			name: "a percentage above a hundred",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingValue = decimal.NewFromInt(150)
			},
			expectedWords: "百分比必須大於零且不超過一百",
		},
		{
			name: "a sizing mode nobody offers",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingMode = "dayTrade"
			},
			expectedWords: "每次開倉押多少只能是",
		},
		{
			name: "a negative stop distance",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.StopLossPercentage = decimal.NewFromInt(-3)
			},
			expectedWords: "停損距離不得為負",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aPositionPlannedBotWriteDto()
			testCase.adjust(&writeDto.PositionPlan)

			_, buildError := domains.NewStrategyBotDomain(writeDto)

			require.Error(t, buildError)
			assert.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedWords)
			assert.NotContains(t, buildError.Error(), "backtest")
		})
	}
}

func TestNewStrategyBotDomainRefusesBorrowing(t *testing.T) {
	testCases := []struct {
		name          string
		leverage      string
		expectsSaving bool
	}{
		{
			name:          "there is nobody here to borrow from",
			leverage:      "1.8",
			expectsSaving: false,
		},
		{
			name:          "suggesting no leverage at all",
			leverage:      "0",
			expectsSaving: true,
		},
		{
			name:          "suggesting one times, which is not a loan",
			leverage:      "1",
			expectsSaving: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aPositionPlannedBotWriteDto()
			writeDto.DeclaredLeverage = decimal.RequireFromString(testCase.leverage)

			_, buildError := domains.NewStrategyBotDomain(writeDto)

			if testCase.expectsSaving {
				require.NoError(t, buildError)
				return
			}

			require.Error(t, buildError)
			assert.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
			// Same wording as the replay, since both use one model.
			assert.Contains(t, buildError.Error(),
				"這個系統只重演現貨，開不了槓桿——現貨是拿現金換東西，沒有人借錢給你")
		})
	}
}

// A negative multiplier is refused, not read as one, now that bots and replays share the
// leverage rule.
func TestNewStrategyBotDomainRefusesANegativeMultiplierRatherThanReadingItAsOne(t *testing.T) {
	writeDto := aPositionPlannedBotWriteDto()
	writeDto.DeclaredLeverage = decimal.RequireFromString("-2")

	_, buildError := domains.NewStrategyBotDomain(writeDto)

	require.Error(t, buildError)
	assert.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
	assert.Contains(t, buildError.Error(), "槓桿倍數不得小於 1 倍")
}

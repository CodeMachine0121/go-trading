package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aBotWriteDto is a bot that passes every rule, so that a test changing one field
// says exactly which rule it is about.
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
	// A saved bot is always stopped. Saving one must not be able to start it.
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
			// A name carrying a NUL cannot be stored by PostgreSQL at all, so it is
			// refused as a bad name here rather than surfacing later as a storage
			// failure nobody can act on.
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
			// A bot is rules plus a machine. Without the rules it is a machine with
			// nothing to do, which is not a bot anybody can start.
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

// A bot carries no rules of its own. It names a set and nothing more, which is what
// lets three bots watch three markets by one set instead of three copies that drift.
func TestNewStrategyBotDomainCarriesNoRulesOfItsOwn(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aBotWriteDto())
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()

	assert.Equal(t, uint(9), botEntity.TradingStrategyID)
	assert.Zero(t, botEntity.TradingStrategy.ID)
}

// A bot's symbol is what its rounds later query with. Lower case is not a mistake
// anybody can see — it reads as the instrument it means — so it is normalised rather
// than refused, and normalised here so that what was written and what is later read
// cannot disagree.
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

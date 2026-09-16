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

// sourceNamed is one signal source declaring one knob, which is enough for every
// rule about sources to have something to bite on.
func sourceNamed(label string, strategyID uint) dto.StrategyBotSignalSourceWriteDto {
	return dto.StrategyBotSignalSourceWriteDto{
		Label:               label,
		StrategyID:          strategyID,
		AggregationInterval: string(vo.AggregationIntervalOneHour),
		DeclaredParameters:  []dto.StrategyParameterWriteDto{{Name: "回看根數", Kind: "lookbackCount"}},
	}
}

// aBotWriteDto is a bot that passes every rule, so that a test changing one field
// says exactly which rule it is about.
func aBotWriteDto() dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		OwnerID:                7,
		Name:                   "早盤突破",
		Symbol:                 "BTCUSDT",
		TriggerIntervalMinutes: 5,
		SignalSources: []dto.StrategyBotSignalSourceWriteDto{
			sourceNamed("A", 1), sourceNamed("B", 2),
		},
		BuyCondition: joining(vo.ConditionOperatorAnd,
			comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
		SellCondition: comparing("A", vo.SignalSell),
	}
}

func TestNewStrategyBotDomainAcceptsAWellFormedBot(t *testing.T) {
	strategyBot, buildError := domains.NewStrategyBotDomain(aBotWriteDto())
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()

	assert.Equal(t, uint(7), botEntity.OwnerID)
	assert.Equal(t, "早盤突破", botEntity.Name)
	assert.Equal(t, "BTCUSDT", botEntity.Symbol)
	assert.Equal(t, 5, botEntity.TriggerIntervalMinutes)
	// A saved bot is always stopped. Saving one must not be able to start it.
	assert.Equal(t, string(vo.StrategyBotStopped), botEntity.RunState)
	assert.Len(t, botEntity.SignalSources, 2)
	require.Len(t, botEntity.ConditionNodes, 2)
	assert.Equal(t, string(vo.StrategyBotConditionSideBuy), botEntity.ConditionNodes[0].Side)
	assert.Equal(t, string(vo.StrategyBotConditionSideSell), botEntity.ConditionNodes[1].Side)
}

func TestNewStrategyBotDomainKeepsNeitherBlanksNorAnIdentifierItWasNotGiven(t *testing.T) {
	writeDto := aBotWriteDto()
	writeDto.Name = "  早盤突破  "
	writeDto.Symbol = "  BTCUSDT  "
	writeDto.SignalSources[0].Label = "  A  "

	strategyBot, buildError := domains.NewStrategyBotDomain(writeDto)
	require.NoError(t, buildError)

	botEntity := strategyBot.ToEntity()
	assert.Equal(t, "早盤突破", botEntity.Name)
	assert.Equal(t, "BTCUSDT", botEntity.Symbol)
	assert.Equal(t, "A", botEntity.SignalSources[0].Label)
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
			name: "a source label longer than allowed",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[0].Label = strings.Repeat("代", 33)
			},
			expectedMessage: "代號長度上限為 32 個字",
		},
		{
			name: "a source label of only blanks",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[0].Label = "   "
			},
			expectedMessage: "都要有一個代號",
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
			name: "no signal sources at all",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources = nil
			},
			expectedMessage: "至少要有一個信號來源",
		},
		{
			name: "two sources answering to one label",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[1].Label = "A"
			},
			expectedMessage: "代號 \"A\" 重複了",
		},
		{
			name: "a source naming no strategy",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[0].StrategyID = 0
			},
			expectedMessage: "必須指名一支策略",
		},
		{
			name: "a coarseness that is not one of the six",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[0].AggregationInterval = "3m"
			},
			expectedMessage: "彙總刻度不對",
		},
		{
			// Caught now rather than at three in the morning, when the same mistake
			// would come back as a script failure and stop the bot.
			name: "a value set on a knob the strategy never declared",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SignalSources[0].ParameterValues = []dto.StrategyParameterValueDto{
					{Name: "週期", Value: 20},
				}
			},
			expectedMessage: "沒有宣告這個名字",
		},
		{
			name: "no buy condition",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.BuyCondition = dto.StrategyBotConditionDto{}
			},
			expectedMessage: "買入條件",
		},
		{
			name: "no sell condition",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.SellCondition = dto.StrategyBotConditionDto{}
			},
			expectedMessage: "賣出條件",
		},
		{
			name: "a condition naming a source this bot never declared",
			mutate: func(writeDto *dto.StrategyBotWriteDto) {
				writeDto.BuyCondition = comparing("D", vo.SignalBuy)
			},
			expectedMessage: "沒有宣告的信號來源 \"D\"",
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

func TestNewStrategyBotDomainRefusesMoreSignalSourcesThanAllowed(t *testing.T) {
	writeDto := aBotWriteDto()
	writeDto.SignalSources = nil
	for index := range 11 {
		writeDto.SignalSources = append(
			writeDto.SignalSources, sourceNamed(string(rune('A'+index)), uint(index+1)))
	}

	_, buildError := domains.NewStrategyBotDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, buildError, "信號來源上限是 10 個")
}

func TestNewStrategyBotDomainLetsOneStrategyBeTwoSources(t *testing.T) {
	writeDto := aBotWriteDto()
	writeDto.SignalSources = []dto.StrategyBotSignalSourceWriteDto{
		sourceNamed("A", 1), sourceNamed("B", 1),
	}
	writeDto.SignalSources[0].ParameterValues = []dto.StrategyParameterValueDto{{Name: "回看根數", Value: 20}}
	writeDto.SignalSources[1].ParameterValues = []dto.StrategyParameterValueDto{{Name: "回看根數", Value: 60}}

	strategyBot, buildError := domains.NewStrategyBotDomain(writeDto)
	require.NoError(t, buildError)

	signalSources := strategyBot.ToEntity().SignalSources
	require.Len(t, signalSources, 2)
	assert.Equal(t, uint(1), signalSources[0].StrategyID)
	assert.Equal(t, uint(1), signalSources[1].StrategyID)
	assert.Equal(t, 20.0, signalSources[0].ParameterValues[0].Value)
	assert.Equal(t, 60.0, signalSources[1].ParameterValues[0].Value)
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

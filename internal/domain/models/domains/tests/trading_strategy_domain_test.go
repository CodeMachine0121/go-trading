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
func sourceNamed(label string, strategyScriptID uint) dto.TradingStrategySignalSourceWriteDto {
	return dto.TradingStrategySignalSourceWriteDto{
		Label:               label,
		StrategyScriptID:    strategyScriptID,
		AggregationInterval: string(vo.AggregationIntervalOneHour),
		DeclaredParameters:  []dto.StrategyScriptParameterWriteDto{{Name: "回看根數", Kind: "lookbackCount"}},
	}
}

// aTradingStrategyWriteDto passes every rule, so that a test changing one field says
// exactly which rule it is about.
func aTradingStrategyWriteDto() dto.TradingStrategyWriteDto {
	return dto.TradingStrategyWriteDto{
		OwnerID: 7,
		Name:    "黃金交叉",
		SignalSources: []dto.TradingStrategySignalSourceWriteDto{
			sourceNamed("A", 1), sourceNamed("B", 2),
		},
		BuyCondition: joining(vo.ConditionOperatorAnd,
			comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy)),
		SellCondition: comparing("A", vo.SignalSell),
	}
}

func TestNewTradingStrategyDomainAcceptsAWellFormedOne(t *testing.T) {
	tradingStrategy, buildError := domains.NewTradingStrategyDomain(aTradingStrategyWriteDto())
	require.NoError(t, buildError)

	tradingStrategyEntity := tradingStrategy.ToEntity()

	assert.Equal(t, uint(7), tradingStrategyEntity.OwnerID)
	assert.Equal(t, "黃金交叉", tradingStrategyEntity.Name)
	assert.Len(t, tradingStrategyEntity.SignalSources, 2)
	require.Len(t, tradingStrategyEntity.ConditionNodes, 2)
	assert.Equal(t,
		string(vo.TradingStrategyConditionSideBuy), tradingStrategyEntity.ConditionNodes[0].Side)
	assert.Equal(t,
		string(vo.TradingStrategyConditionSideSell), tradingStrategyEntity.ConditionNodes[1].Side)
}

func TestNewTradingStrategyDomainKeepsNoBlanks(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.Name = "  黃金交叉  "
	writeDto.SignalSources[0].Label = "  A  "

	tradingStrategy, buildError := domains.NewTradingStrategyDomain(writeDto)
	require.NoError(t, buildError)

	tradingStrategyEntity := tradingStrategy.ToEntity()
	assert.Equal(t, "黃金交叉", tradingStrategyEntity.Name)
	assert.Equal(t, "A", tradingStrategyEntity.SignalSources[0].Label)
}

// A set of rules holds no market, no interval and no run state. Those describe a
// machine following it, and a shape with nowhere to put them is what lets several
// machines follow one set.
func TestNewTradingStrategyDomainHoldsNothingAboutAnyMachine(t *testing.T) {
	tradingStrategy, buildError := domains.NewTradingStrategyDomain(aTradingStrategyWriteDto())
	require.NoError(t, buildError)

	tradingStrategyEntity := tradingStrategy.ToEntity()

	assert.Zero(t, tradingStrategyEntity.ID)
	assert.NotEmpty(t, tradingStrategyEntity.SignalSources)
	assert.NotEmpty(t, tradingStrategyEntity.ConditionNodes)
}

func TestNewTradingStrategyDomainRefusals(t *testing.T) {
	testCases := []struct {
		name            string
		mutate          func(writeDto *dto.TradingStrategyWriteDto)
		expectedMessage string
	}{
		{
			name:            "one with nobody behind it",
			mutate:          func(writeDto *dto.TradingStrategyWriteDto) { writeDto.OwnerID = 0 },
			expectedMessage: "必須屬於一位使用者",
		},
		{
			name:            "a name of only blanks",
			mutate:          func(writeDto *dto.TradingStrategyWriteDto) { writeDto.Name = "   " },
			expectedMessage: "取一個名稱",
		},
		{
			name: "a name one character over the limit",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.Name = strings.Repeat("名", 129)
			},
			expectedMessage: "長度上限為 128 個字",
		},
		{
			// A name carrying a NUL cannot be stored by PostgreSQL at all, so it is
			// refused as a bad name here rather than surfacing later as a storage
			// failure nobody can act on.
			name:            "a name carrying a null character",
			mutate:          func(writeDto *dto.TradingStrategyWriteDto) { writeDto.Name = "黃金\x00交叉" },
			expectedMessage: "不得包含空字元",
		},
		{
			name: "a source label longer than allowed",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[0].Label = strings.Repeat("代", 33)
			},
			expectedMessage: "代號長度上限為 32 個字",
		},
		{
			name: "a source label of only blanks",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[0].Label = "   "
			},
			expectedMessage: "都要有一個代號",
		},
		{
			name: "no signal sources at all",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources = nil
			},
			expectedMessage: "至少要有一個信號來源",
		},
		{
			name: "two sources answering to one label",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[1].Label = "A"
			},
			expectedMessage: "代號 \"A\" 重複了",
		},
		{
			name: "a source naming no strategy script",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[0].StrategyScriptID = 0
			},
			expectedMessage: "必須指名一支策略腳本",
		},
		{
			name: "a coarseness that is not one of the six",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[0].AggregationInterval = "3m"
			},
			expectedMessage: "彙總刻度不對",
		},
		{
			// Caught now rather than at three in the morning, when the same mistake
			// would come back as a script failure and stop every bot following this.
			name: "a value set on a knob the strategy script never declared",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SignalSources[0].ParameterValues = []dto.StrategyScriptParameterValueDto{
					{Name: "週期", Value: 20},
				}
			},
			expectedMessage: "沒有宣告這個名字",
		},
		{
			name: "no buy condition",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.BuyCondition = dto.TradingStrategyConditionDto{}
			},
			expectedMessage: "買入條件",
		},
		{
			name: "no sell condition",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.SellCondition = dto.TradingStrategyConditionDto{}
			},
			expectedMessage: "賣出條件",
		},
		{
			name: "a condition naming a source this trading strategy never declared",
			mutate: func(writeDto *dto.TradingStrategyWriteDto) {
				writeDto.BuyCondition = comparing("D", vo.SignalBuy)
			},
			expectedMessage: "沒有宣告的信號來源 \"D\"",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aTradingStrategyWriteDto()
			testCase.mutate(&writeDto)

			_, buildError := domains.NewTradingStrategyDomain(writeDto)

			require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestNewTradingStrategyDomainRefusesMoreSignalSourcesThanAllowed(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources = nil
	for index := range 11 {
		writeDto.SignalSources = append(
			writeDto.SignalSources, sourceNamed(string(rune('A'+index)), uint(index+1)))
	}

	_, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, buildError, "信號來源上限是 10 個")
}

func TestNewTradingStrategyDomainLetsOneStrategyScriptBeTwoSources(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources = []dto.TradingStrategySignalSourceWriteDto{
		sourceNamed("A", 1), sourceNamed("B", 1),
	}
	writeDto.SignalSources[0].ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "回看根數", Value: 20}}
	writeDto.SignalSources[1].ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "回看根數", Value: 60}}

	tradingStrategy, buildError := domains.NewTradingStrategyDomain(writeDto)
	require.NoError(t, buildError)

	signalSources := tradingStrategy.ToEntity().SignalSources
	require.Len(t, signalSources, 2)
	assert.Equal(t, uint(1), signalSources[0].StrategyScriptID)
	assert.Equal(t, uint(1), signalSources[1].StrategyScriptID)
	assert.Equal(t, 20.0, signalSources[0].ParameterValues[0].Value)
	assert.Equal(t, 60.0, signalSources[1].ParameterValues[0].Value)
}

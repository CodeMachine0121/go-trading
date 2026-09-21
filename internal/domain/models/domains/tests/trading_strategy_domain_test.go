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

// sourceNamed is one signal source declaring one knob and naming a strategy script
// that speaks in signals, which is enough for every rule about sources to have
// something to bite on.
func sourceNamed(label string, strategyScriptID uint) dto.TradingStrategySignalSourceWriteDto {
	return dto.TradingStrategySignalSourceWriteDto{
		Label:               label,
		StrategyScriptID:    strategyScriptID,
		AggregationInterval: string(vo.AggregationIntervalOneHour),
		DeclaredParameters:  []dto.StrategyScriptParameterWriteDto{{Name: "回看根數", Kind: "lookbackCount"}},
		DeclaredResultType:  string(vo.IndicatorResultTypeSignal),
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

// A condition compares a source against buy, sell or hold, and only a strategy script
// declared as a signal ever produces those. Naming any other kind builds a set of
// rules whose every sentence has nothing on the other side of it — and nothing would
// say so until the bot woke up in the night and stopped on a script failure.
func TestNewTradingStrategyDomainRefusesASourceThatDoesNotSpeakInSignals(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		resultType string
	}{
		{name: "一個數字", resultType: string(vo.IndicatorResultTypeFloat)},
		{name: "一串數字", resultType: string(vo.IndicatorResultTypeFloatList)},
		{name: "一個是非", resultType: string(vo.IndicatorResultTypeBool)},
		{name: "一串是非", resultType: string(vo.IndicatorResultTypeBoolList)},
		{name: "什麼都沒宣告", resultType: ""},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aTradingStrategyWriteDto()
			writeDto.SignalSources[0].DeclaredResultType = testCase.resultType

			_, buildError := domains.NewTradingStrategyDomain(writeDto)

			require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
			// The refusal names the source, because a trading strategy may hold ten
			// of them and "one of them is wrong" is not something anybody can act on.
			assert.ErrorContains(t, buildError, "A")
			assert.ErrorContains(t, buildError, "signal")
		})
	}
}

// A trading strategy whose sources read different coarsenesses cannot do anything:
// replaying it is refused, and once it is running two sources on two different clocks
// have their opinions read as two sentences about the same candle. Refusing it here
// means the person finds out while they are still looking at the form.
func TestNewTradingStrategyDomainRefusesSourcesThatReadDifferentCoarseness(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources[1].AggregationInterval = string(vo.AggregationIntervalFiveMinutes)

	_, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	// The refusal names them. Told only that they differ, somebody has to open every
	// source to see how — and making them the same is the thing they then have to do.
	assert.ErrorContains(t, buildError, "1h")
	assert.ErrorContains(t, buildError, "5m")
}

func TestNewTradingStrategyDomainNamesEveryCoarsenessItFound(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources = append(writeDto.SignalSources, sourceNamed("C", 3))
	writeDto.SignalSources[2].AggregationInterval = string(vo.AggregationIntervalOneDay)
	writeDto.BuyCondition = joining(vo.ConditionOperatorAnd,
		comparing("A", vo.SignalBuy), comparing("B", vo.SignalBuy))

	_, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, buildError, "1h")
	assert.ErrorContains(t, buildError, "1d")
}

// One source cannot disagree with anybody, so the rule has nothing to say about it.
func TestNewTradingStrategyDomainAcceptsASingleSourceWhateverItReads(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.SignalSources = []dto.TradingStrategySignalSourceWriteDto{sourceNamed("A", 1)}
	writeDto.SignalSources[0].AggregationInterval = string(vo.AggregationIntervalFiveMinutes)
	writeDto.BuyCondition = comparing("A", vo.SignalBuy)

	tradingStrategy, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.NoError(t, buildError)
	assert.Equal(t,
		string(vo.AggregationIntervalFiveMinutes),
		tradingStrategy.ToEntity().SignalSources[0].AggregationInterval)
}

func TestNewTradingStrategyDomainReadsTheTradingMode(t *testing.T) {
	testCases := []struct {
		name         string
		declaredMode string
		expectedMode vo.TradingModeVo
	}{
		{
			name:         "declared spot",
			declaredMode: "spot",
			expectedMode: vo.TradingModeSpot,
		},
		{
			name:         "declared long-short",
			declaredMode: "longShort",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			// Word for word what a replay does with a blank one, because there is one
			// model that knows what a trading mode may be and both ask it.
			name:         "declared nothing at all",
			declaredMode: "",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			name:         "declared blanks",
			declaredMode: "   ",
			expectedMode: vo.TradingModeLongShort,
		},
		{
			name:         "the spelling is read however it was typed",
			declaredMode: "SPOT",
			expectedMode: vo.TradingModeSpot,
		},
		{
			// Rules for an account that never shorts and borrows anyway. It reaches
			// this model the way the other two do, and had to be walked once rather
			// than left to the mode model's own tests: what is being checked here is
			// that a trading strategy stores it, not that the mode can be read.
			name:         "declared long only but borrowing",
			declaredMode: "leveragedLong",
			expectedMode: vo.TradingModeLeveragedLong,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aTradingStrategyWriteDto()
			writeDto.TradingMode = testCase.declaredMode

			tradingStrategy, buildError := domains.NewTradingStrategyDomain(writeDto)

			require.NoError(t, buildError)
			assert.Equal(t,
				string(testCase.expectedMode), tradingStrategy.ToEntity().TradingMode)
		})
	}
}

func TestNewTradingStrategyDomainRefusesATradingModeItCannotRead(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.TradingMode = "dayTrade"

	_, buildError := domains.NewTradingStrategyDomain(writeDto)

	require.Error(t, buildError)
	// The one sentinel every refused trading strategy carries, so that a controller
	// maps this without learning a second one.
	assert.ErrorIs(t, buildError, domains.ErrTradingStrategyValidation)
	// Every spelling is offered back, in the very sentence a replay's refusal uses:
	// a caller who reaches one refusal has already read the other.
	//
	// Asserted against the whole selectable set rather than against a list written
	// out here. A mode added to the system and forgotten by this refusal leaves
	// somebody unable to discover it from the one message that was supposed to say
	// what is allowed — and a hand-written list of two stayed green through exactly
	// that, which is why it is derived now.
	for _, selectableMode := range []vo.TradingModeVo{
		vo.TradingModeLongShort, vo.TradingModeSpot, vo.TradingModeLeveragedLong,
	} {
		assert.Contains(t, buildError.Error(), string(selectableMode))
	}
}

func TestNewTradingStrategyDomainRefusesAnUnreadableModeInTheSameWordsAReplayDoes(t *testing.T) {
	writeDto := aTradingStrategyWriteDto()
	writeDto.TradingMode = "dayTrade"

	_, saveError := domains.NewTradingStrategyDomain(writeDto)
	_, modeError := domains.NewTradingModeDomain("dayTrade")

	require.Error(t, saveError)
	require.Error(t, modeError)
	// Not merely similar: the reason is the mode's own sentence, carried through. Two
	// separately worded lists of what a trading mode may be would eventually disagree,
	// and a person reading one of them would be told something untrue.
	assert.Contains(t, saveError.Error(), modeError.Error())
}

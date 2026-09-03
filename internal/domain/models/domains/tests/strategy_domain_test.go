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

// strategyMaxCandleCount is the ceiling these tests judge a candle count against.
// It matches the single-query maximum the application is configured with.
const strategyMaxCandleCount = 1000

// aStrategyWriteDto is a strategy that passes every rule, so that each test can
// break exactly one thing and nothing else explains the outcome.
func aStrategyWriteDto() dto.StrategyWriteDto {
	return dto.StrategyWriteDto{
		Name:                "二十根均線",
		Script:              "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType:          "floatList",
		AggregationInterval: "1h",
		CandleCount:         20,
	}
}

func TestNewStrategyDomainKeepsWhatItWasGiven(t *testing.T) {
	strategyDomain, validationError := domains.NewStrategyDomain(
		aStrategyWriteDto(), strategyMaxCandleCount)

	require.NoError(t, validationError)

	strategy := strategyDomain.ToEntity()
	assert.Equal(t, "二十根均線", strategy.Name)
	assert.Equal(t, aStrategyWriteDto().Script, strategy.Script)
	assert.Equal(t, "floatList", strategy.ResultType)
	assert.Equal(t, "1h", strategy.AggregationInterval)
	assert.Equal(t, 20, strategy.CandleCount)
}

func TestNewStrategyDomainAppliesTheSameDefaultsAsEverywhereElse(t *testing.T) {
	testCases := []struct {
		name                        string
		resultType                  string
		aggregationInterval         string
		expectedResultType          string
		expectedAggregationInterval string
	}{
		{
			name:       "declaring no aggregation interval means five minutes",
			resultType: "floatList", aggregationInterval: "",
			expectedResultType: "floatList", expectedAggregationInterval: "5m",
		},
		{
			name:       "declaring no result type means one number",
			resultType: "", aggregationInterval: "1h",
			expectedResultType: "float", expectedAggregationInterval: "1h",
		},
		{
			name:       "declaring neither falls back on both",
			resultType: "", aggregationInterval: "",
			expectedResultType: "float", expectedAggregationInterval: "5m",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			writeDto.ResultType = testCase.resultType
			writeDto.AggregationInterval = testCase.aggregationInterval

			strategyDomain, validationError := domains.NewStrategyDomain(
				writeDto, strategyMaxCandleCount)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedResultType, strategyDomain.ToEntity().ResultType)
			assert.Equal(t,
				testCase.expectedAggregationInterval, strategyDomain.ToEntity().AggregationInterval)
		})
	}
}

func TestNewStrategyDomainNamesAreJudgedWithoutTheBlanksAroundThem(t *testing.T) {
	testCases := []struct {
		name         string
		declaredName string
		expectedName string
	}{
		{
			name:         "the blanks around a name are not part of it",
			declaredName: "　二十根均線　",
			expectedName: "二十根均線",
		},
		{
			name:         "a name of exactly the maximum length is allowed",
			declaredName: strings.Repeat("均", 128),
			expectedName: strings.Repeat("均", 128),
		},
		{
			name:         "length is counted after the blanks are dropped",
			declaredName: "  " + strings.Repeat("均", 128) + "  ",
			expectedName: strings.Repeat("均", 128),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			writeDto.Name = testCase.declaredName

			strategyDomain, validationError := domains.NewStrategyDomain(
				writeDto, strategyMaxCandleCount)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedName, strategyDomain.ToEntity().Name)
		})
	}
}

func TestNewStrategyDomainRefusesContentThatBreaksARule(t *testing.T) {
	testCases := []struct {
		name            string
		breakIt         func(writeDto *dto.StrategyWriteDto)
		expectedMessage string
	}{
		{
			name:            "no name at all",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Name = "" },
			expectedMessage: "必須給策略取一個名稱",
		},
		{
			name:            "a name of nothing but blanks is no name",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Name = "  　 " },
			expectedMessage: "必須給策略取一個名稱",
		},
		{
			name: "a name one character over the limit",
			breakIt: func(writeDto *dto.StrategyWriteDto) {
				writeDto.Name = strings.Repeat("均", 129)
			},
			expectedMessage: "策略名稱長度上限為 128 個字",
		},
		{
			name:            "no script",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Script = "" },
			expectedMessage: "策略必須帶一段指標算式",
		},
		{
			name:            "a script of nothing but blanks",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Script = "   \n\t " },
			expectedMessage: "策略必須帶一段指標算式",
		},
		{
			name:            "an aggregation interval nobody offers",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.AggregationInterval = "7m" },
			expectedMessage: "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一",
		},
		{
			name:            "an aggregation interval that would not divide a day",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.AggregationInterval = "1w" },
			expectedMessage: "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一",
		},
		{
			name:            "a result type nobody offers",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.ResultType = "string" },
			expectedMessage: "指標值種類只能是 float、floatList、bool、boolList 其中之一",
		},
		{
			name:            "a candle count of zero",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.CandleCount = 0 },
			expectedMessage: "計算根數必須大於零",
		},
		{
			name:            "a negative candle count",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.CandleCount = -1 },
			expectedMessage: "計算根數必須大於零",
		},
		{
			name:            "a candle count one over the ceiling",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.CandleCount = 1001 },
			expectedMessage: "超過單次可用的最大根數（最多 1000 根）",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			testCase.breakIt(&writeDto)

			_, validationError := domains.NewStrategyDomain(writeDto, strategyMaxCandleCount)

			require.ErrorIs(t, validationError, domains.ErrStrategyValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessage)
		})
	}
}

func TestNewStrategyDomainAcceptsACandleCountUpToTheCeiling(t *testing.T) {
	for _, candleCount := range []int{1, 20, 999, 1000} {
		writeDto := aStrategyWriteDto()
		writeDto.CandleCount = candleCount

		strategyDomain, validationError := domains.NewStrategyDomain(
			writeDto, strategyMaxCandleCount)

		require.NoError(t, validationError)
		assert.Equal(t, candleCount, strategyDomain.ToEntity().CandleCount)
	}
}

func TestNewStrategyDomainJudgesTheCandleCountApartFromTheInterval(t *testing.T) {
	// Both look back one day. The ceiling is about how many candles are read, not
	// about how much time they cover, so a coarse interval buys no extra room and a
	// fine one costs none.
	testCases := []struct {
		aggregationInterval string
		candleCount         int
	}{
		{aggregationInterval: "5m", candleCount: 288},
		{aggregationInterval: "1h", candleCount: 24},
	}

	for _, testCase := range testCases {
		t.Run(testCase.aggregationInterval, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			writeDto.AggregationInterval = testCase.aggregationInterval
			writeDto.CandleCount = testCase.candleCount

			_, validationError := domains.NewStrategyDomain(writeDto, strategyMaxCandleCount)

			require.NoError(t, validationError)
		})
	}
}

func TestNewStrategyDomainSavesAScriptItCannotVouchFor(t *testing.T) {
	// Saving is not running. An algorithm takes several sittings to get right, so a
	// half-finished one has to be storable or there is no way to pick it up tomorrow.
	writeDto := aStrategyWriteDto()
	writeDto.Script = "這根本不是一段程式碼 ¯\\_(ツ)_/¯"

	strategyDomain, validationError := domains.NewStrategyDomain(
		writeDto, strategyMaxCandleCount)

	require.NoError(t, validationError)
	assert.Equal(t, writeDto.Script, strategyDomain.ToEntity().Script)
}

func TestNewStrategyDomainCarriesTheIdentifierItWasNamedBy(t *testing.T) {
	testCases := []struct {
		name       string
		id         uint
		expectedID uint
	}{
		{name: "no identifier means a strategy that does not exist yet", id: 0, expectedID: 0},
		{name: "an identifier names the strategy being rewritten", id: 7, expectedID: 7},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			writeDto.ID = testCase.id

			strategyDomain, validationError := domains.NewStrategyDomain(
				writeDto, strategyMaxCandleCount)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedID, strategyDomain.ToEntity().ID)
		})
	}
}

func TestNewStrategyDomainHandsOutTheIntervalAndKindAlreadyRead(t *testing.T) {
	// Whatever runs this strategy needs what the interval knows — how long a bucket
	// is, how many stored candles it holds — not the spelling of it. Handing out the
	// spelling would invite that to be worked out a second time.
	writeDto := aStrategyWriteDto()
	writeDto.AggregationInterval = "1h"
	writeDto.ResultType = "boolList"

	strategyDomain, validationError := domains.NewStrategyDomain(
		writeDto, strategyMaxCandleCount)

	require.NoError(t, validationError)
	assert.Equal(t, vo.AggregationIntervalOneHour, strategyDomain.AggregationInterval().Value())
	assert.Equal(t, 12, strategyDomain.AggregationInterval().SourceCandleCount(1))
	assert.Equal(t, vo.IndicatorResultTypeBoolList, strategyDomain.ResultType().Value())
	assert.True(t, strategyDomain.ResultType().IsList())
}

func TestNewStrategyDomainToEntityLeavesTheTimesToWhoeverSavesIt(t *testing.T) {
	strategyDomain, validationError := domains.NewStrategyDomain(
		aStrategyWriteDto(), strategyMaxCandleCount)

	require.NoError(t, validationError)
	assert.True(t, strategyDomain.ToEntity().CreatedAt.IsZero())
	assert.True(t, strategyDomain.ToEntity().UpdatedAt.IsZero())
}

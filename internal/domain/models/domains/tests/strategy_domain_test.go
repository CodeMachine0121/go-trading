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

// aStrategyWriteDto is a strategy that passes every rule, so that each test can
// break exactly one thing and nothing else explains the outcome.
func aStrategyWriteDto() dto.StrategyWriteDto {
	return dto.StrategyWriteDto{
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "floatList",
	}
}

func TestNewStrategyDomainKeepsWhatItWasGiven(t *testing.T) {
	strategyDomain, validationError := domains.NewStrategyDomain(aStrategyWriteDto())

	require.NoError(t, validationError)

	strategy := strategyDomain.ToEntity()
	assert.Equal(t, "二十根均線", strategy.Name)
	assert.Equal(t, aStrategyWriteDto().Script, strategy.Script)
	assert.Equal(t, "floatList", strategy.ResultType)
}

func TestNewStrategyDomainAppliesTheSameDefaultAsEverywhereElse(t *testing.T) {
	// Declaring no result type means one number, exactly as a calculation that
	// declares none gets one number. One rule, not two that have to be kept in step.
	writeDto := aStrategyWriteDto()
	writeDto.ResultType = ""

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, "float", strategyDomain.ToEntity().ResultType)
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

			strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

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
			name:            "a result type nobody offers",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.ResultType = "string" },
			expectedMessage: "指標值種類只能是 float、floatList、bool、boolList 其中之一",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyWriteDto()
			testCase.breakIt(&writeDto)

			_, validationError := domains.NewStrategyDomain(writeDto)

			require.ErrorIs(t, validationError, domains.ErrStrategyValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessage)
		})
	}
}

func TestNewStrategyDomainSavesAScriptItCannotVouchFor(t *testing.T) {
	// Saving is not running. An algorithm takes several sittings to get right, so a
	// half-finished one has to be storable or there is no way to pick it up tomorrow.
	writeDto := aStrategyWriteDto()
	writeDto.Script = "這根本不是一段程式碼 ¯\\_(ツ)_/¯"

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, writeDto.Script, strategyDomain.ToEntity().Script)
}

func TestNewStrategyDomainSavesAScriptThatContradictsItsDeclaredKind(t *testing.T) {
	// Whether a script's shape matches the kind it was declared under is decided
	// when the script runs, against the candles it was handed. Saving cannot know it
	// and does not pretend to.
	writeDto := aStrategyWriteDto()
	writeDto.ResultType = "bool"
	writeDto.Script = "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }"

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, "bool", strategyDomain.ToEntity().ResultType)
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

			strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedID, strategyDomain.ToEntity().ID)
		})
	}
}

func TestNewStrategyDomainHandsOutTheKindAlreadyRead(t *testing.T) {
	// Whatever runs this strategy needs what the kind knows — whether it is a list —
	// not the spelling of it. Handing out the spelling would invite that to be
	// worked out a second time.
	writeDto := aStrategyWriteDto()
	writeDto.ResultType = "boolList"

	strategyDomain, validationError := domains.NewStrategyDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, vo.IndicatorResultTypeBoolList, strategyDomain.ResultType().Value())
	assert.True(t, strategyDomain.ResultType().IsList())
}

func TestNewStrategyDomainToEntityLeavesTheTimesToWhoeverSavesIt(t *testing.T) {
	strategyDomain, validationError := domains.NewStrategyDomain(aStrategyWriteDto())

	require.NoError(t, validationError)
	assert.True(t, strategyDomain.ToEntity().CreatedAt.IsZero())
	assert.True(t, strategyDomain.ToEntity().UpdatedAt.IsZero())
}

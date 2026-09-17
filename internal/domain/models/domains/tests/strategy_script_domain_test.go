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

// strategyScriptWriteOwnerID is who these strategy scripts belong to. Every strategy script has an
// owner now, so a write without one is refused before any other rule is read —
// which would make every case below fail for the wrong reason.
const strategyScriptWriteOwnerID = uint(1)

// aStrategyScriptWriteDto is a strategy script that passes every rule, so that each test can
// break exactly one thing and nothing else explains the outcome.
func aStrategyScriptWriteDto() dto.StrategyScriptWriteDto {
	return dto.StrategyScriptWriteDto{
		OwnerID:    strategyScriptWriteOwnerID,
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "floatList",
	}
}

func TestNewStrategyScriptDomainKeepsWhatItWasGiven(t *testing.T) {
	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(aStrategyScriptWriteDto())

	require.NoError(t, validationError)

	strategyScript := strategyScriptDomain.ToEntity()
	assert.Equal(t, "二十根均線", strategyScript.Name)
	assert.Equal(t, aStrategyScriptWriteDto().Script, strategyScript.Script)
	assert.Equal(t, "floatList", strategyScript.ResultType)
}

func TestNewStrategyScriptDomainAppliesTheSameDefaultAsEverywhereElse(t *testing.T) {
	// Declaring no result type means one number, exactly as a calculation that
	// declares none gets one number. One rule, not two that have to be kept in step.
	writeDto := aStrategyScriptWriteDto()
	writeDto.ResultType = ""

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, "float", strategyScriptDomain.ToEntity().ResultType)
}

func TestNewStrategyScriptDomainNamesAreJudgedWithoutTheBlanksAroundThem(t *testing.T) {
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
			writeDto := aStrategyScriptWriteDto()
			writeDto.Name = testCase.declaredName

			strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedName, strategyScriptDomain.ToEntity().Name)
		})
	}
}

func TestNewStrategyScriptDomainRefusesContentThatBreaksARule(t *testing.T) {
	testCases := []struct {
		name            string
		breakIt         func(writeDto *dto.StrategyScriptWriteDto)
		expectedMessage string
	}{
		{
			name:            "no name at all",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Name = "" },
			expectedMessage: "必須給策略腳本取一個名稱",
		},
		{
			name:            "a name of nothing but blanks is no name",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Name = "  　 " },
			expectedMessage: "必須給策略腳本取一個名稱",
		},
		{
			name: "a name one character over the limit",
			breakIt: func(writeDto *dto.StrategyScriptWriteDto) {
				writeDto.Name = strings.Repeat("均", 129)
			},
			expectedMessage: "策略腳本名稱長度上限為 128 個字",
		},
		{
			name:            "no script",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Script = "" },
			expectedMessage: "策略腳本必須帶一段指標算式",
		},
		{
			name:            "a script of nothing but blanks",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Script = "   \n\t " },
			expectedMessage: "策略腳本必須帶一段指標算式",
		},
		{
			name:            "a result type nobody offers",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.ResultType = "string" },
			expectedMessage: "指標值種類只能是 float、floatList、bool、boolList、signal 其中之一",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyScriptWriteDto()
			testCase.breakIt(&writeDto)

			_, validationError := domains.NewStrategyScriptDomain(writeDto)

			require.ErrorIs(t, validationError, domains.ErrStrategyScriptValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessage)
		})
	}
}

func TestNewStrategyScriptDomainSavesAScriptItCannotVouchFor(t *testing.T) {
	// Saving is not running. An algorithm takes several sittings to get right, so a
	// half-finished one has to be storable or there is no way to pick it up tomorrow.
	writeDto := aStrategyScriptWriteDto()
	writeDto.Script = "這根本不是一段程式碼 ¯\\_(ツ)_/¯"

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, writeDto.Script, strategyScriptDomain.ToEntity().Script)
}

func TestNewStrategyScriptDomainSavesAScriptThatContradictsItsDeclaredKind(t *testing.T) {
	// Whether a script's shape matches the kind it was declared under is decided
	// when the script runs, against the candles it was handed. Saving cannot know it
	// and does not pretend to.
	writeDto := aStrategyScriptWriteDto()
	writeDto.ResultType = "bool"
	writeDto.Script = "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }"

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, "bool", strategyScriptDomain.ToEntity().ResultType)
	assert.Equal(t, writeDto.Script, strategyScriptDomain.ToEntity().Script)
}

func TestNewStrategyScriptDomainCarriesTheIdentifierItWasNamedBy(t *testing.T) {
	testCases := []struct {
		name       string
		id         uint
		expectedID uint
	}{
		{name: "no identifier means a strategy script that does not exist yet", id: 0, expectedID: 0},
		{name: "an identifier names the strategy script being rewritten", id: 7, expectedID: 7},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aStrategyScriptWriteDto()
			writeDto.ID = testCase.id

			strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedID, strategyScriptDomain.ToEntity().ID)
		})
	}
}

func TestNewStrategyScriptDomainAcceptsTheSignalKind(t *testing.T) {
	t.Run("creating a strategy script that emits signals", func(t *testing.T) {
		writeDto := aStrategyScriptWriteDto()
		writeDto.ResultType = "signal"

		strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

		require.NoError(t, validationError)
		assert.Equal(t, "signal", strategyScriptDomain.ToEntity().ResultType)
		assert.True(t, strategyScriptDomain.ResultType().IsSignal())
	})

	t.Run("rewriting an existing strategy script to emit signals", func(t *testing.T) {
		writeDto := aStrategyScriptWriteDto()
		writeDto.ID = 7
		writeDto.ResultType = "signal"

		strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

		require.NoError(t, validationError)
		assert.Equal(t, uint(7), strategyScriptDomain.ToEntity().ID)
		assert.True(t, strategyScriptDomain.ResultType().IsSignal())
	})
}

func TestNewStrategyScriptDomainHandsOutTheKindAlreadyRead(t *testing.T) {
	// Whatever runs this strategy script needs what the kind knows — whether it is a list —
	// not the spelling of it. Handing out the spelling would invite that to be
	// worked out a second time.
	writeDto := aStrategyScriptWriteDto()
	writeDto.ResultType = "boolList"

	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(writeDto)

	require.NoError(t, validationError)
	assert.Equal(t, vo.IndicatorResultTypeBoolList, strategyScriptDomain.ResultType().Value())
	assert.True(t, strategyScriptDomain.ResultType().IsList())
}

func TestNewStrategyScriptDomainToEntityLeavesTheTimesToWhoeverSavesIt(t *testing.T) {
	strategyScriptDomain, validationError := domains.NewStrategyScriptDomain(aStrategyScriptWriteDto())

	require.NoError(t, validationError)
	assert.True(t, strategyScriptDomain.ToEntity().CreatedAt.IsZero())
	assert.True(t, strategyScriptDomain.ToEntity().UpdatedAt.IsZero())
}

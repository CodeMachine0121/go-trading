package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func lookbackCountParameter(name string, defaultValue float64) dto.StrategyParameterWriteDto {
	return dto.StrategyParameterWriteDto{Name: name, Kind: "lookbackCount", DefaultValue: defaultValue}
}

func numberParameter(name string, defaultValue float64) dto.StrategyParameterWriteDto {
	return dto.StrategyParameterWriteDto{Name: name, Kind: "number", DefaultValue: defaultValue}
}

func booleanParameter(name string, defaultValue float64) dto.StrategyParameterWriteDto {
	return dto.StrategyParameterWriteDto{Name: name, Kind: "boolean", DefaultValue: defaultValue}
}

func parametersOf(t *testing.T, declared ...dto.StrategyParameterWriteDto) domains.StrategyParametersDomain {
	t.Helper()

	parameters, buildError := domains.NewStrategyParametersDomain(declared)
	require.NoError(t, buildError)

	return parameters
}

// 建立一份參數就是把整份規則走過一遍：實例存在就代表這一份是可用的。
func TestDeclaringParametersRefusesWhatCannotBeOne(t *testing.T) {
	testCases := []struct {
		name             string
		declared         []dto.StrategyParameterWriteDto
		expectedFragment string
	}{
		{
			name:             "同一支策略內名稱不得重複",
			declared:         []dto.StrategyParameterWriteDto{lookbackCountParameter("期數", 20), numberParameter("期數", 2)},
			expectedFragment: "重複",
		},
		{
			name:             "名稱不得為空白",
			declared:         []dto.StrategyParameterWriteDto{lookbackCountParameter("   ", 20)},
			expectedFragment: "不得為空白",
		},
		{
			name:             "回看根數必須大於零",
			declared:         []dto.StrategyParameterWriteDto{lookbackCountParameter("期數", 0)},
			expectedFragment: "大於零的整數",
		},
		{
			name:             "回看根數必須是整數",
			declared:         []dto.StrategyParameterWriteDto{lookbackCountParameter("期數", 20.5)},
			expectedFragment: "大於零的整數",
		},
		{
			name:             "名稱太長",
			declared:         []dto.StrategyParameterWriteDto{lookbackCountParameter(strings.Repeat("長", 65), 20)},
			expectedFragment: "超過 64 個字",
		},
		{
			name: "種類只有那兩種",
			declared: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "somethingElse", DefaultValue: 20}},
			expectedFragment: "不在可宣告的種類之內",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewStrategyParametersDomain(testCase.declared)

			require.ErrorIs(t, buildError, domains.ErrStrategyParameterValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedFragment)
		})
	}
}

func TestDeclaringParametersAcceptsWhatItShould(t *testing.T) {
	t.Run("名稱前後的空白不予保留", func(t *testing.T) {
		parameters := parametersOf(t, lookbackCountParameter("  期數  ", 20))

		lookbackCount, isDeclared := parameters.LookbackCountOf("期數")
		assert.True(t, isDeclared)
		assert.Equal(t, 20, lookbackCount)
	})

	t.Run("數值不限正負與小數——系統本來就不解讀它", func(t *testing.T) {
		parameters := parametersOf(t, numberParameter("偏移", -1.5))

		number, isDeclared := parameters.NumberOf("偏移")
		assert.True(t, isDeclared)
		assert.InDelta(t, -1.5, number, 0)
	})

	t.Run("一個參數都不宣告是合法的一份", func(t *testing.T) {
		parameters := parametersOf(t)

		assert.Equal(t, 0, parameters.MaximumLookbackCount())
		assert.Empty(t, parameters.ToDtos())
	})
}

// 這個數字是「要拿幾根」唯一的依據，拿錯就是拿錯量的 K 線。
func TestMaximumLookbackCountLooksOnlyAtLookbackCounts(t *testing.T) {
	testCases := []struct {
		name     string
		declared []dto.StrategyParameterWriteDto
		expected int
	}{
		{name: "一個都沒有就是零", declared: nil, expected: 0},
		{
			name:     "只有數值也是零——數值跟拿幾根無關",
			declared: []dto.StrategyParameterWriteDto{numberParameter("倍數", 999)},
			expected: 0,
		},
		{
			name:     "只看最大的那一個",
			declared: []dto.StrategyParameterWriteDto{lookbackCountParameter("快線", 20), lookbackCountParameter("慢線", 100), lookbackCountParameter("中線", 50)},
			expected: 100,
		},
		{
			name:     "數值再大也不算進來",
			declared: []dto.StrategyParameterWriteDto{lookbackCountParameter("期數", 20), numberParameter("倍數", 5000)},
			expected: 20,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, parametersOf(t, testCase.declared...).MaximumLookbackCount())
		})
	}
}

// 套值是唯一的入口，它一次答完四件事，所以呼叫端不必自己排、也漏不掉其中一步。
func TestApplyingThisRunsValues(t *testing.T) {
	t.Run("給了值就用給的", func(t *testing.T) {
		applied, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying([]dto.StrategyParameterValueDto{{Name: "期數", Value: 50}})

		require.NoError(t, applyError)
		lookbackCount, _ := applied.LookbackCountOf("期數")
		assert.Equal(t, 50, lookbackCount)
	})

	t.Run("沒給值就用宣告的預設值", func(t *testing.T) {
		applied, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying(nil)

		require.NoError(t, applyError)
		lookbackCount, _ := applied.LookbackCountOf("期數")
		assert.Equal(t, 20, lookbackCount)
	})

	t.Run("給了沒有宣告的名稱就整次拒絕", func(t *testing.T) {
		// 安靜忽略會讓人以為他調的那一格有作用，而它什麼都沒做。
		_, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying([]dto.StrategyParameterValueDto{{Name: "週期", Value: 50}})

		require.ErrorIs(t, applyError, domains.ErrStrategyParameterValidation)
		assert.Contains(t, applyError.Error(), "週期")
	})

	t.Run("給的回看根數不合法就整次拒絕", func(t *testing.T) {
		_, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying([]dto.StrategyParameterValueDto{{Name: "期數", Value: 0}})

		require.ErrorIs(t, applyError, domains.ErrStrategyParameterValidation)
		assert.Contains(t, applyError.Error(), "大於零的整數")
	})

	t.Run("套值之後最大回看根數跟著變——要拿幾根就是照這個算的", func(t *testing.T) {
		applied, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying([]dto.StrategyParameterValueDto{{Name: "期數", Value: 100}})

		require.NoError(t, applyError)
		assert.Equal(t, 100, applied.MaximumLookbackCount())
	})

	t.Run("沒有人取用的參數值不是錯誤", func(t *testing.T) {
		// 系統沒有、也不該有辦法知道算式用了哪些名字。
		applied, applyError := parametersOf(t, lookbackCountParameter("期數", 20)).
			Applying([]dto.StrategyParameterValueDto{{Name: "期數", Value: 50}})

		require.NoError(t, applyError)
		assert.Len(t, applied.ToDtos(), 1)
	})
}

// 這第二個回傳值就是「名字對不上」與「算式壞了」分得開的原因。
func TestReadingAParameterSaysWhetherItWasDeclared(t *testing.T) {
	parameters := parametersOf(t, lookbackCountParameter("期數", 20), numberParameter("倍數", 2))

	t.Run("宣告過的回得出值", func(t *testing.T) {
		lookbackCount, isDeclared := parameters.LookbackCountOf("期數")
		assert.True(t, isDeclared)
		assert.Equal(t, 20, lookbackCount)

		number, isNumberDeclared := parameters.NumberOf("倍數")
		assert.True(t, isNumberDeclared)
		assert.InDelta(t, 2.0, number, 0)
	})

	t.Run("沒宣告過的說得出它沒宣告", func(t *testing.T) {
		_, isDeclared := parameters.LookbackCountOf("週期")
		assert.False(t, isDeclared)

		_, isNumberDeclared := parameters.NumberOf("週期")
		assert.False(t, isNumberDeclared)
	})
}

// 是非仍然是一個數字，種類只說怎麼讀它——這是分種類的整個意義，
// 也是為什麼多一種讀法不必動到值是怎麼存、怎麼送的。
func TestABooleanIsOneNumberReadAsYesOrNo(t *testing.T) {
	t.Run("零是否，非零是是", func(t *testing.T) {
		parameters := parametersOf(t,
			booleanParameter("要濾掉假突破", 1), booleanParameter("只看多方", 0))

		isTrue, isDeclared := parameters.BooleanOf("要濾掉假突破")
		assert.True(t, isDeclared)
		assert.True(t, isTrue)

		isFalse, isFalseDeclared := parameters.BooleanOf("只看多方")
		assert.True(t, isFalseDeclared)
		assert.False(t, isFalse)
	})

	t.Run("沒宣告過的說得出它沒宣告", func(t *testing.T) {
		parameters := parametersOf(t, booleanParameter("要濾掉假突破", 1))

		_, isDeclared := parameters.BooleanOf("只看多方")
		assert.False(t, isDeclared)
	})

	t.Run("宣告時就把值收成剛好零或一", func(t *testing.T) {
		// 存下來的東西要自己說得出它是哪一個，而不是留一個 0.7 給下一個讀的人解讀。
		parameters := parametersOf(t, booleanParameter("要濾掉假突破", 0.7))

		assert.InDelta(t, 1.0, parameters.ToDtos()[0].DefaultValue, 0)
	})

	t.Run("這一次改成別的數字，收回來的仍然是零或一", func(t *testing.T) {
		parameters := parametersOf(t, booleanParameter("要濾掉假突破", 1))

		applied, applyError := parameters.Applying(
			[]dto.StrategyParameterValueDto{{Name: "要濾掉假突破", Value: 2}})

		require.NoError(t, applyError)
		assert.InDelta(t, 1.0, applied.ToDtos()[0].DefaultValue, 0)
	})

	t.Run("它不是回看根數，所以不影響要拿幾根", func(t *testing.T) {
		// 回看根數是系統唯一會解讀的那一種。多一種讀法不該讓它多讀一根 K 線。
		parameters := parametersOf(t, booleanParameter("要濾掉假突破", 1))

		assert.Equal(t, 0, parameters.MaximumLookbackCount())
	})

	t.Run("零不會被當成不合法——是非沒有大於零這回事", func(t *testing.T) {
		_, buildError := domains.NewStrategyParametersDomain(
			[]dto.StrategyParameterWriteDto{booleanParameter("只看多方", 0)})

		assert.NoError(t, buildError)
	})
}

package domains_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func distinctSymbols(count int) string {
	symbols := make([]string, 0, count)
	for index := range count {
		symbols = append(symbols, fmt.Sprintf("COIN%dUSDT", index))
	}

	return strings.Join(symbols, ",")
}

func TestSymbolScreeningSymbolListTidiesWhatWasPasted(t *testing.T) {
	testCases := []struct {
		name            string
		pasted          string
		expectedSymbols []string
	}{
		{name: "case and full-width commas", pasted: "xrpusdt, DOGEUSDT，linkusdt",
			expectedSymbols: []string{"XRPUSDT", "DOGEUSDT", "LINKUSDT"}},
		{name: "repeats and blanks", pasted: "ADAUSDT,adausdt,,", expectedSymbols: []string{"ADAUSDT"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			symbolList, listError := domains.NewSymbolScreeningSymbolListDomain(testCase.pasted, 20, false)

			require.NoError(t, listError)
			assert.Equal(t, testCase.expectedSymbols, symbolList.Symbols())
		})
	}
}

func TestSymbolScreeningSymbolListRefusesWhatCannotBeScreened(t *testing.T) {
	testCases := []struct {
		name            string
		pasted          string
		expectedMessage string
	}{
		{name: "nothing left after tidying", pasted: " , ，", expectedMessage: "沒有任何標的"},
		{name: "one over the limit", pasted: distinctSymbols(21), expectedMessage: "一次最多 20 個標的"},
		{name: "a dash", pasted: "BTC-USDT", expectedMessage: "BTC-USDT 只能有英文字母與數字"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, listError := domains.NewSymbolScreeningSymbolListDomain(testCase.pasted, 20, false)

			require.ErrorIs(t, listError, domains.ErrSymbolScreeningValidation)
			assert.Contains(t, listError.Error(), testCase.expectedMessage)
		})
	}
}

func TestSymbolScreeningSymbolListAcceptsExactlyTheLimit(t *testing.T) {
	symbolList, listError := domains.NewSymbolScreeningSymbolListDomain(distinctSymbols(20), 20, false)

	require.NoError(t, listError)
	assert.Len(t, symbolList.Symbols(), 20)
}

func TestSymbolScreeningSymbolListMayBeEmptyForABaseline(t *testing.T) {
	symbolList, listError := domains.NewSymbolScreeningSymbolListDomain("", 10, true)

	require.NoError(t, listError)
	assert.Empty(t, symbolList.Symbols())
}

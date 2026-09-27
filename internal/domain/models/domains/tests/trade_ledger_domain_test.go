package domains_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	ledgerNow          = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)
	errLedgerViolation = errors.New("ledger violation")
	testLedgerWording  = vo.TradeLedgerWordingVo{
		Entry: "買進", Exit: "賣出", Holding: "持有", Price: "價格", OverExitAdvice: "（試算）",
		ValidationError: errLedgerViolation,
	}
)

func ledgerEntry(id uint, filledAt time.Time, price string, quantity string) vo.TradeLedgerFillVo {
	return vo.TradeLedgerFillVo{
		ID: id, Kind: vo.ContractTradeFillKindEntry, FilledAt: filledAt,
		Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
	}
}

func ledgerExit(id uint, filledAt time.Time, price string, quantity string) vo.TradeLedgerFillVo {
	fill := ledgerEntry(id, filledAt, price, quantity)
	fill.Kind = vo.ContractTradeFillKindExit

	return fill
}

func ledgerOf(fills ...vo.TradeLedgerFillVo) domains.TradeLedgerDomain {
	return domains.NewTradeLedgerDomain(fills, testLedgerWording)
}

func TestTradeLedgerDomainAveragesAndPositions(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

	t.Run("adding to a holding recomputes the average entry", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "97905", "0.030"),
			ledgerEntry(2, firstEntryAt.Add(8*time.Minute), "97960", "0.021"))

		require.NoError(t, ledger.Validate(ledgerNow))
		assert.Equal(t, "0.051", ledger.Position().String())
		assert.Equal(t, "97927.6", ledger.AverageEntryPrice().StringFixed(1))
		assert.Equal(t, "0.051", ledger.EnteredQuantity().String())
	})

	t.Run("a partial exit keeps the trade open", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "97905", "0.051"),
			ledgerExit(2, firstEntryAt.Add(time.Hour), "99000", "0.020"))

		require.NoError(t, ledger.Validate(ledgerNow))
		assert.Equal(t, "0.031", ledger.Position().String())
		assert.False(t, ledger.IsFlat())
	})

	t.Run("exiting exactly what is held leaves it flat", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "97905", "0.031"),
			ledgerExit(2, firstEntryAt.Add(time.Hour), "99000", "0.031"))

		require.NoError(t, ledger.Validate(ledgerNow))
		assert.True(t, ledger.IsFlat())
		assert.True(t, ledger.LastFillAt().Equal(firstEntryAt.Add(time.Hour)))
	})

	t.Run("an entry and an exit at the same instant read as entry first", func(t *testing.T) {
		ledger := ledgerOf(ledgerExit(2, firstEntryAt, "101", "1"), ledgerEntry(1, firstEntryAt, "100", "1"))

		require.NoError(t, ledger.Validate(ledgerNow))
		assert.True(t, ledger.IsFlat())
		assert.Equal(t, "100", ledger.FirstEntryPrice().String())
	})

	t.Run("the position at a moment counts fills at that moment", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "100", "2"), ledgerExit(2, firstEntryAt.Add(time.Hour), "101", "1"))

		assert.Equal(t, "0", ledger.PositionAt(firstEntryAt.Add(-time.Second)).String())
		assert.Equal(t, "2", ledger.PositionAt(firstEntryAt).String())
		assert.Equal(t, "1", ledger.PositionAt(firstEntryAt.Add(time.Hour)).String())
	})

	t.Run("fees add up across fills", func(t *testing.T) {
		entry := ledgerEntry(1, firstEntryAt, "100", "2")
		entry.Fee = decimal.RequireFromString("1.5")
		exit := ledgerExit(2, firstEntryAt.Add(time.Hour), "101", "2")
		exit.Fee = decimal.RequireFromString("0.5")

		assert.Equal(t, "2", ledgerOf(entry, exit).TotalFee().String())
		assert.Equal(t, "200", ledgerOf(entry, exit).EntryValue().String())
	})
}

func TestTradeLedgerDomainRefusesFillsThatBreakTheTradeInTheJournalsWords(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)
	held := ledgerEntry(1, firstEntryAt, "97905", "0.031")

	negativeFee := ledgerEntry(2, firstEntryAt.Add(time.Hour), "99000", "0.01")
	negativeFee.Fee = decimal.RequireFromString("-1")
	unknownKind := ledgerEntry(2, firstEntryAt.Add(time.Hour), "99000", "0.01")
	unknownKind.Kind = "reverse"

	testCases := []struct {
		name            string
		fills           []vo.TradeLedgerFillVo
		expectedMessage string
	}{
		{name: "selling back more than is held", fills: []vo.TradeLedgerFillVo{held,
			ledgerExit(2, firstEntryAt.Add(time.Hour), "99000", "0.050")},
			expectedMessage: "賣出數量超過目前持有 0.031（試算）"},
		{name: "a zero price", fills: []vo.TradeLedgerFillVo{held, ledgerExit(2, firstEntryAt.Add(time.Hour), "0", "0.01")},
			expectedMessage: "價格與數量必須大於零"},
		{name: "a negative quantity", fills: []vo.TradeLedgerFillVo{held,
			ledgerExit(2, firstEntryAt.Add(time.Hour), "99000", "-0.01")}, expectedMessage: "價格與數量必須大於零"},
		{name: "a fill in the future", fills: []vo.TradeLedgerFillVo{held,
			ledgerExit(2, ledgerNow.Add(2*time.Hour), "99000", "0.01")}, expectedMessage: "時間不能在未來"},
		{name: "an exit before the first entry", fills: []vo.TradeLedgerFillVo{held,
			ledgerExit(2, firstEntryAt.Add(-63*time.Minute), "99000", "0.01")}, expectedMessage: "賣出不能早於第一筆買進"},
		{name: "a kind that is neither entry nor exit", fills: []vo.TradeLedgerFillVo{held, unknownKind},
			expectedMessage: "只有買進與賣出"},
		{name: "a negative fee", fills: []vo.TradeLedgerFillVo{held, negativeFee}, expectedMessage: "手續費不得為負"},
		{name: "no entry left", fills: nil, expectedMessage: "一筆交易至少要有一筆買進；要整筆放棄請刪除交易"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			validationError := ledgerOf(testCase.fills...).Validate(ledgerNow)

			require.ErrorIs(t, validationError, errLedgerViolation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessage)
		})
	}
}

func TestTradeLedgerDomainProfits(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

	t.Run("a long's realised profit is measured from the average entry", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "97905", "0.030"),
			ledgerEntry(2, firstEntryAt.Add(8*time.Minute), "97960", "0.021"),
			ledgerExit(3, firstEntryAt.Add(26*time.Hour), "100420", "0.051"))

		assert.Equal(t, "127.11", ledger.GrossProfit(vo.PositionDirectionLong).StringFixed(2))
		averageExitPrice, hasExited := ledger.AverageExitPrice()
		assert.True(t, hasExited)
		assert.Equal(t, "100420", averageExitPrice.String())
	})

	t.Run("a short loses when the price rises", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "3500", "1"), ledgerExit(2, firstEntryAt.Add(time.Hour), "3550", "1"))

		assert.Equal(t, "-50", ledger.GrossProfit(vo.PositionDirectionShort).String())
	})

	t.Run("nothing sold back has no realised profit and no exit average", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "3500", "1"))

		_, hasExited := ledger.AverageExitPrice()
		assert.False(t, hasExited)
		assert.True(t, ledger.GrossProfit(vo.PositionDirectionLong).IsZero())
	})

	t.Run("an open short gains as the price falls", func(t *testing.T) {
		ledger := ledgerOf(ledgerEntry(1, firstEntryAt, "3500", "2"))

		assert.Equal(t, "20", ledger.OpenProfitAt(vo.PositionDirectionShort, decimal.RequireFromString("3490")).String())
		assert.Equal(t, "20", ledger.ProfitAt(vo.PositionDirectionShort, decimal.RequireFromString("3490")).String())
		assert.Equal(t, "-20", ledger.OpenProfitAt(vo.PositionDirectionLong, decimal.RequireFromString("3490")).String())
	})
}

func TestTradeLedgerDomainWithoutFills(t *testing.T) {
	ledger := ledgerOf()

	assert.True(t, ledger.FirstEntryAt().IsZero())
	assert.True(t, ledger.FirstEntryPrice().IsZero())
	assert.True(t, ledger.LastFillAt().IsZero())
	assert.True(t, ledger.AverageEntryPrice().IsZero())
}

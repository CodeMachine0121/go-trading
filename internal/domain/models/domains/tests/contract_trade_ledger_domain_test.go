package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var ledgerNow = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

func entryFill(id uint, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	return entities.ContractTradeFill{
		ID: id, Kind: string(vo.ContractTradeFillKindEntry), FilledAt: filledAt,
		Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
		Liquidity: string(vo.TradeFillLiquidityTaker),
	}
}

func exitFill(id uint, filledAt time.Time, price string, quantity string) entities.ContractTradeFill {
	fill := entryFill(id, filledAt, price, quantity)
	fill.Kind = string(vo.ContractTradeFillKindExit)

	return fill
}

func TestContractTradeLedgerDomainAveragesAndPositions(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

	t.Run("adding to a long recomputes the average entry", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain(
			[]entities.ContractTradeFill{entryFill(1, firstEntryAt, "97905", "0.030")}).
			Admit(entryFill(0, firstEntryAt.Add(8*time.Minute), "97960", "0.021"), ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "0.051", ledger.Position().String())
		assert.Equal(t, "97927.6", ledger.AverageEntryPrice().StringFixed(1))
	})

	t.Run("a partial exit keeps the trade open", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.051"),
		}).Admit(exitFill(0, firstEntryAt.Add(time.Hour), "99000", "0.020"), ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "0.031", ledger.Position().String())
		assert.False(t, ledger.IsFlat())
	})

	t.Run("exiting exactly the position leaves it flat", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.031"),
		}).Admit(exitFill(0, firstEntryAt.Add(time.Hour), "99000", "0.031"), ledgerNow)

		require.NoError(t, err)
		assert.True(t, ledger.IsFlat())
		assert.True(t, ledger.LastFillAt().Equal(firstEntryAt.Add(time.Hour)))
	})

	t.Run("an entry and an exit at the same instant read as entry first", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain(nil).Admit(
			entryFill(0, firstEntryAt, "100", "1"), ledgerNow)
		require.NoError(t, err)

		ledger, err = ledger.Admit(exitFill(0, firstEntryAt, "101", "1"), ledgerNow)

		require.NoError(t, err)
		assert.True(t, ledger.IsFlat())
	})

	t.Run("the position at a moment counts fills at that moment", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "100", "2"),
			exitFill(2, firstEntryAt.Add(time.Hour), "101", "1"),
		})

		assert.Equal(t, "0", ledger.PositionAt(firstEntryAt.Add(-time.Second)).String())
		assert.Equal(t, "2", ledger.PositionAt(firstEntryAt).String())
		assert.Equal(t, "1", ledger.PositionAt(firstEntryAt.Add(time.Hour)).String())
	})
}

func TestContractTradeLedgerDomainRefusesFillsThatBreakTheTrade(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)
	heldLedger := domains.NewContractTradeLedgerDomain(
		[]entities.ContractTradeFill{entryFill(1, firstEntryAt, "97905", "0.031")})

	zeroPrice := exitFill(0, firstEntryAt.Add(time.Hour), "0", "0.01")
	negativeQuantity := exitFill(0, firstEntryAt.Add(time.Hour), "99000", "-0.01")
	unknownKind := entryFill(0, firstEntryAt.Add(time.Hour), "99000", "0.01")
	unknownKind.Kind = "reverse"
	unknownLiquidity := entryFill(0, firstEntryAt.Add(time.Hour), "99000", "0.01")
	unknownLiquidity.Liquidity = "iceberg"
	negativeFee := entryFill(0, firstEntryAt.Add(time.Hour), "99000", "0.01")
	negativeFee.Fee = decimal.RequireFromString("-1")

	testCases := []struct {
		name            string
		fill            entities.ContractTradeFill
		expectedMessage string
	}{
		{name: "selling back more than is held", fill: exitFill(0, firstEntryAt.Add(time.Hour), "99000", "0.050"),
			expectedMessage: "出場數量超過目前持倉 0.031，要反手請先平倉再新增一筆反方向的交易"},
		{name: "a zero price", fill: zeroPrice, expectedMessage: "成交價與數量必須大於零"},
		{name: "a negative quantity", fill: negativeQuantity, expectedMessage: "成交價與數量必須大於零"},
		{name: "a fill in the future", fill: exitFill(0, ledgerNow.Add(2*time.Hour), "99000", "0.01"),
			expectedMessage: "成交時間不能在未來"},
		{name: "an exit before the first entry", fill: exitFill(0, firstEntryAt.Add(-63*time.Minute), "99000", "0.01"),
			expectedMessage: "出場不能早於第一筆進場"},
		{name: "a kind that is neither entry nor exit", fill: unknownKind, expectedMessage: "成交只有進場（entry）與出場（exit）"},
		{name: "a liquidity that is neither maker nor taker", fill: unknownLiquidity, expectedMessage: "成交方式只有掛單（maker）與吃單（taker）"},
		{name: "a negative fee", fill: negativeFee, expectedMessage: "手續費不得為負"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := heldLedger.Admit(testCase.fill, ledgerNow)

			require.ErrorIs(t, err, domains.ErrContractTradeValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestContractTradeLedgerDomainAmendsAndRemovesFills(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

	t.Run("a mistyped entry is corrected and the average follows", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97095", "0.030"),
		}).Amend(1, entryFill(0, firstEntryAt, "97905", "0.030"), ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "97905", ledger.AverageEntryPrice().String())
		assert.Equal(t, uint(1), ledger.Fills()[0].ID)
	})

	t.Run("removing the only entry is refused", func(t *testing.T) {
		_, err := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.030"),
		}).Remove(1, ledgerNow)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "一筆交易至少要有一筆進場成交；要整筆放棄請刪除交易")
	})

	t.Run("removing one of two entries is allowed", func(t *testing.T) {
		ledger, err := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.030"),
			entryFill(2, firstEntryAt.Add(time.Minute), "97960", "0.021"),
		}).Remove(2, ledgerNow)

		require.NoError(t, err)
		assert.Equal(t, "0.03", ledger.Position().String())
	})

	t.Run("a fill the trade does not have cannot be amended or removed", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.030"),
		})

		_, amendError := ledger.Amend(9, entryFill(0, firstEntryAt, "1", "1"), ledgerNow)
		_, removeError := ledger.Remove(9, ledgerNow)

		require.ErrorIs(t, amendError, domains.ErrContractTradeValidation)
		require.ErrorIs(t, removeError, domains.ErrContractTradeValidation)
		assert.Contains(t, amendError.Error(), "這筆交易沒有識別碼為 9 的成交")
	})
}

func TestContractTradeLedgerDomainProfits(t *testing.T) {
	firstEntryAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

	t.Run("a long's realised profit is measured from the average entry", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "97905", "0.030"),
			entryFill(2, firstEntryAt.Add(8*time.Minute), "97960", "0.021"),
			exitFill(3, firstEntryAt.Add(26*time.Hour), "100420", "0.051"),
		})

		assert.Equal(t, "127.11", ledger.GrossProfit(vo.PositionDirectionLong).StringFixed(2))
		averageExitPrice, hasExited := ledger.AverageExitPrice()
		assert.True(t, hasExited)
		assert.Equal(t, "100420", averageExitPrice.String())
	})

	t.Run("a short loses when the price rises", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "3500", "1"),
			exitFill(2, firstEntryAt.Add(time.Hour), "3550", "1"),
		})

		assert.Equal(t, "-50", ledger.GrossProfit(vo.PositionDirectionShort).String())
	})

	t.Run("nothing sold back has no realised profit and no exit average", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "3500", "1"),
		})

		_, hasExited := ledger.AverageExitPrice()
		assert.False(t, hasExited)
		assert.True(t, ledger.GrossProfit(vo.PositionDirectionLong).IsZero())
	})

	t.Run("an open short gains as the price falls", func(t *testing.T) {
		ledger := domains.NewContractTradeLedgerDomain([]entities.ContractTradeFill{
			entryFill(1, firstEntryAt, "3500", "2"),
		})

		assert.Equal(t, "20", ledger.OpenProfitAt(vo.PositionDirectionShort, decimal.RequireFromString("3490")).String())
		assert.Equal(t, "20", ledger.ProfitAt(vo.PositionDirectionShort, decimal.RequireFromString("3490")).String())
	})
}

func TestContractTradeLedgerDomainWithoutFills(t *testing.T) {
	ledger := domains.NewContractTradeLedgerDomain(nil)

	assert.True(t, ledger.FirstEntryAt().IsZero())
	assert.True(t, ledger.FirstEntryPrice().IsZero())
	assert.True(t, ledger.LastFillAt().IsZero())
	assert.True(t, ledger.AverageEntryPrice().IsZero())
}

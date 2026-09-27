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

// theBitcoinLong is the worked example: two entries averaging 97,927.6, one exit at 100,420, stop at 96,380.
func theBitcoinLong() entities.ContractTradeRecord {
	closedAt := tradeOpenedAt.Add(26*time.Hour + 37*time.Minute)
	firstFill := entryFill(1, tradeOpenedAt, "97905", "0.030")
	firstFill.Fee = decimal.RequireFromString("1.47")
	secondFill := entryFill(2, tradeOpenedAt.Add(8*time.Minute), "97960", "0.021")
	secondFill.Fee = decimal.RequireFromString("1.03")
	closingFill := exitFill(3, closedAt, "100420", "0.051")
	closingFill.Fee = decimal.RequireFromString("2.56")

	return entities.ContractTradeRecord{
		ID: 27, Direction: "long", Leverage: decimal.NewFromInt(10), Status: string(vo.ContractTradeStatusClosed),
		PlannedStopLossPrice: price("96380"), OpenedAt: tradeOpenedAt, ClosedAt: &closedAt,
		Fills: []entities.ContractTradeFill{firstFill, secondFill, closingFill},
	}
}

func threeSettlements(fundingRate string, markPrice string) []vo.FundingSettlementVo {
	settlements := []vo.FundingSettlementVo{}
	for hour := 16; hour <= 32; hour += 8 {
		settlements = append(settlements, vo.FundingSettlementVo{
			SettlementTime: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC).Add(time.Duration(hour) * time.Hour),
			FundingRate:    decimal.RequireFromString(fundingRate),
			MarkPrice:      price(markPrice),
		})
	}

	return settlements
}

func journalOutcomeOf(record entities.ContractTradeRecord, facts vo.ContractTradeMarketFactsVo) domains.ContractTradeOutcomeDomain {
	return domains.NewContractTradeOutcomeDomain(domains.NewContractTradeRecordDomain(record), facts)
}

func TestContractTradeOutcomeDomainMoney(t *testing.T) {
	t.Run("the closed long's profit, risk and R", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{
			FundingSettlements: threeSettlements("0.0001", "99020"), FundingSettlementDue: true,
		}).Outcome()

		assert.Equal(t, "127.11", outcome.GrossProfit.StringFixed(2))
		assert.Equal(t, "5.06", outcome.TotalFee.StringFixed(2))
		assert.Equal(t, "-1.52", outcome.Funding.Amount.StringFixed(2))
		assert.Equal(t, 3, outcome.Funding.SettlementCount)
		assert.Equal(t, "120.53", outcome.NetProfit.StringFixed(2))
		assert.Equal(t, "78.93", outcome.PlannedRisk.Decimal.StringFixed(2))
		require.NotNil(t, outcome.RMultiple)
		assert.InDelta(t, 1.53, *outcome.RMultiple, 0.005)
		assert.False(t, outcome.NetProfitExcludesFunding)
	})

	t.Run("a short receives funding when the rate is positive", func(t *testing.T) {
		record := theBitcoinLong()
		record.Direction = "short"
		record.PlannedStopLossPrice = decimal.NullDecimal{}

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{
			FundingSettlements: threeSettlements("0.0001", "99020"), FundingSettlementDue: true,
		}).Outcome()

		assert.Equal(t, "1.52", outcome.Funding.Amount.StringFixed(2))
	})

	t.Run("a short's risk runs up to its stop", func(t *testing.T) {
		closedAt := tradeOpenedAt.Add(time.Hour)
		record := entities.ContractTradeRecord{
			Direction: "short", Leverage: decimal.NewFromInt(5), Status: string(vo.ContractTradeStatusClosed),
			PlannedStopLossPrice: price("3550"), ClosedAt: &closedAt,
			Fills: []entities.ContractTradeFill{
				entryFill(1, tradeOpenedAt, "3500", "1"), exitFill(2, closedAt, "3450", "1"),
			},
		}

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "50", outcome.PlannedRisk.Decimal.String())
		assert.InDelta(t, 1.0, *outcome.RMultiple, 0.0001)
	})

	t.Run("no stop means no R, with the reason", func(t *testing.T) {
		record := theBitcoinLong()
		record.PlannedStopLossPrice = decimal.NullDecimal{}

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Nil(t, outcome.RMultiple)
		assert.False(t, outcome.PlannedRisk.Valid)
		assert.Equal(t, "noStopLoss", outcome.RMultipleUnavailableReason)
		assert.Equal(t, "127.11", outcome.GrossProfit.StringFixed(2))
	})

	t.Run("no settlement time inside the holding means no funding", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{FundingSettlementDue: false}).Outcome()

		assert.True(t, outcome.Funding.Available)
		assert.True(t, outcome.Funding.Amount.IsZero())
	})

	t.Run("missing settlement data is said, never zero", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{FundingSettlementDue: true}).Outcome()

		assert.False(t, outcome.Funding.Available)
		assert.Equal(t, "noSettlementData", outcome.Funding.UnavailableReason)
		assert.True(t, outcome.NetProfitExcludesFunding)
		assert.Equal(t, "122.05", outcome.NetProfit.StringFixed(2))
	})

	t.Run("a settlement before any entry charges nothing and a missing mark uses the entry average", func(t *testing.T) {
		settlements := []vo.FundingSettlementVo{
			{SettlementTime: tradeOpenedAt.Add(-time.Hour), FundingRate: decimal.RequireFromString("0.01")},
			{SettlementTime: tradeOpenedAt.Add(time.Hour), FundingRate: decimal.RequireFromString("0.0001")},
		}

		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{
			FundingSettlements: settlements, FundingSettlementDue: true}).Outcome()

		assert.Equal(t, 1, outcome.Funding.SettlementCount)
		assert.Equal(t, "-0.50", outcome.Funding.Amount.StringFixed(2))
	})

	t.Run("a zero fee from a missing rate is flagged", func(t *testing.T) {
		record := theBitcoinLong()
		record.Fills[0].FeeRateMissing = true

		assert.True(t, journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).Outcome().FeeRateMissing)
	})
}

func TestContractTradeOutcomeDomainExcursions(t *testing.T) {
	t.Run("adverse and favourable runs in R and the share captured", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{
			ExtremesRequested: true,
			PriceExtremes: vo.PriceExtremesVo{
				LowestPrice: decimal.RequireFromString("97110"), HighestPrice: decimal.RequireFromString("100960"), Has: true,
			},
		}).Outcome()

		require.True(t, outcome.Excursion.Available)
		assert.InDelta(t, -0.53, *outcome.Excursion.AdverseRMultiple, 0.005)
		assert.InDelta(t, 1.96, *outcome.Excursion.FavorableRMultiple, 0.005)
		require.NotNil(t, outcome.ProfitCaptureRate)
		assert.InDelta(t, 0.82, *outcome.ProfitCaptureRate, 0.005)
	})

	t.Run("a short's adverse run is the high", func(t *testing.T) {
		record := theBitcoinLong()
		record.Direction = "short"

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{
			ExtremesRequested: true,
			PriceExtremes: vo.PriceExtremesVo{
				LowestPrice: decimal.RequireFromString("97110"), HighestPrice: decimal.RequireFromString("100960"), Has: true,
			},
		}).Outcome()

		assert.Equal(t, "100960", outcome.Excursion.AdversePrice.String())
		require.NotNil(t, outcome.ProfitCaptureRate)
		assert.InDelta(t, -3.05, *outcome.ProfitCaptureRate, 0.005)
	})

	t.Run("no candles while held is said", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{ExtremesRequested: true}).Outcome()

		assert.False(t, outcome.Excursion.Available)
		assert.Equal(t, "noMarketData", outcome.Excursion.UnavailableReason)
		assert.Nil(t, outcome.ProfitCaptureRate)
	})

	t.Run("a trade still held has no share captured yet", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{
			ExtremesRequested: true,
			PriceExtremes: vo.PriceExtremesVo{
				LowestPrice: decimal.RequireFromString("97110"), HighestPrice: decimal.RequireFromString("100960"), Has: true,
			},
		}).Outcome()

		require.True(t, outcome.Excursion.Available)
		assert.Nil(t, outcome.ProfitCaptureRate)
	})

	t.Run("a summary does not work excursions out", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "notComputed", outcome.Excursion.UnavailableReason)
	})
}

func openBitcoinLong() entities.ContractTradeRecord {
	return entities.ContractTradeRecord{
		ID: 31, Direction: "long", Leverage: decimal.NewFromInt(10), Status: string(vo.ContractTradeStatusOpen),
		OpenedAt: tradeOpenedAt,
		Fills: []entities.ContractTradeFill{
			entryFill(1, tradeOpenedAt, "97905", "0.030"),
			entryFill(2, tradeOpenedAt.Add(8*time.Minute), "97960", "0.021"),
		},
	}
}

func bitcoinTradingRules(t *testing.T) domains.ContractTradingRulesDomain {
	specifiedAt := ledgerNow
	tradingRules, err := domains.NewContractTradingRulesDomain(entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", TickSize: price("0.1"), MaintenanceMarginRate: price("0.005"),
		SpecificationUpdatedAt: &specifiedAt,
	}, true, nil)
	require.NoError(t, err)

	return tradingRules
}

func TestContractTradeOutcomeDomainOpenPosition(t *testing.T) {
	t.Run("the floating profit at the latest price", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{
			LatestPrice: decimal.RequireFromString("98500"), HasLatestPrice: true,
		}).Outcome()

		require.True(t, outcome.FloatingProfit.Available)
		assert.Equal(t, "29.19", outcome.FloatingProfit.Amount.StringFixed(2))
		assert.Equal(t, "98500", outcome.FloatingProfit.Price.String())
	})

	t.Run("no latest price means no floating profit", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "noLatestPrice", outcome.FloatingProfit.UnavailableReason)
	})

	t.Run("a closed trade has no floating profit or liquidation price", func(t *testing.T) {
		outcome := journalOutcomeOf(theBitcoinLong(), vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "notOpen", outcome.FloatingProfit.UnavailableReason)
		assert.Equal(t, "notOpen", outcome.LiquidationPrice.UnavailableReason)
	})

	t.Run("the liquidation estimate follows the replay's isolated-margin formula", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{}).
			WithTradingRules(bitcoinTradingRules(t), true).Outcome()

		require.True(t, outcome.LiquidationPrice.Available)
		assert.Equal(t, "88577.8", outcome.LiquidationPrice.Price.String())
	})

	t.Run("a one-times long cannot be liquidated", func(t *testing.T) {
		record := openBitcoinLong()
		record.Leverage = decimal.NewFromInt(1)

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).
			WithTradingRules(bitcoinTradingRules(t), true).Outcome()

		assert.True(t, outcome.LiquidationPrice.CannotBeLiquidated)
	})

	t.Run("a short's liquidation lies above its entry", func(t *testing.T) {
		record := openBitcoinLong()
		record.Direction = "short"

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).
			WithTradingRules(bitcoinTradingRules(t), true).Outcome()

		assert.True(t, outcome.LiquidationPrice.Price.GreaterThan(decimal.RequireFromString("97927.6")))
	})

	t.Run("no specification means no estimate", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{}).
			WithTradingRules(domains.ContractTradingRulesDomain{}, false).Outcome()

		assert.Equal(t, "noTradingSpecification", outcome.LiquidationPrice.UnavailableReason)
	})

	t.Run("a summary does not estimate liquidation", func(t *testing.T) {
		outcome := journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "notComputed", outcome.LiquidationPrice.UnavailableReason)
	})
}

func TestContractTradeOutcomeDomainEntrySlippage(t *testing.T) {
	t.Run("a long filled above the reference slipped", func(t *testing.T) {
		record := openBitcoinLong()
		record.Fills = record.Fills[:1]
		record.SourceReferencePrice = price("97850")

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).Outcome()

		require.NotNil(t, outcome.EntrySlippagePercentage)
		assert.InDelta(t, 0.06, *outcome.EntrySlippagePercentage, 0.005)
	})

	t.Run("a short filled below the reference slipped", func(t *testing.T) {
		record := entities.ContractTradeRecord{
			Direction: "short", Leverage: decimal.NewFromInt(5), Status: string(vo.ContractTradeStatusOpen),
			SourceReferencePrice: price("3500"),
			Fills:                []entities.ContractTradeFill{entryFill(1, tradeOpenedAt, "3493", "1")},
		}

		outcome := journalOutcomeOf(record, vo.ContractTradeMarketFactsVo{}).Outcome()

		require.NotNil(t, outcome.EntrySlippagePercentage)
		assert.InDelta(t, 0.20, *outcome.EntrySlippagePercentage, 0.0001)
	})

	t.Run("a trade not started from a bot round has no slippage", func(t *testing.T) {
		assert.Nil(t, journalOutcomeOf(openBitcoinLong(), vo.ContractTradeMarketFactsVo{}).Outcome().EntrySlippagePercentage)
	})
}

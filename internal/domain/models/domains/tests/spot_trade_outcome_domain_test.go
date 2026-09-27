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

func spotRecordOf(status string, plannedStopLossPrice decimal.NullDecimal, fills ...entities.SpotTradeFill) domains.SpotTradeRecordDomain {
	return domains.NewSpotTradeRecordDomain(entities.SpotTradeRecord{
		Market: "taiwanStock", Status: status, PlannedStopLossPrice: plannedStopLossPrice, Fills: fills,
	})
}

func TestSpotTradeOutcomeDomain(t *testing.T) {
	soldAt := spotBoughtAt.Add(26 * time.Hour)
	withFee := func(fill entities.SpotTradeFill, fee string) entities.SpotTradeFill {
		fill.Fee = decimal.RequireFromString(fee)
		return fill
	}

	t.Run("a closed trade's profit, return and R", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", price("1000"),
			withFee(spotBuy(1, spotBoughtAt, "1050", "1000"), "1496"),
			withFee(spotSell(2, soldAt, "1120", "1000"), "604"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "70000", outcome.GrossProfit.String())
		assert.Equal(t, "2100", outcome.TotalFee.String())
		assert.Equal(t, "67900", outcome.NetProfit.String())
		assert.Equal(t, "1050000", outcome.BuyCost.String())
		require.NotNil(t, outcome.ReturnRate)
		assert.InDelta(t, 0.0647, *outcome.ReturnRate, 0.00005)
		require.NotNil(t, outcome.RMultiple)
		assert.InDelta(t, 1.36, *outcome.RMultiple, 0.005)
		assert.Empty(t, outcome.RMultipleUnavailableReason)
	})

	t.Run("without a planned stop the return stands and R cannot be worked out", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", decimal.NullDecimal{},
			spotBuy(1, spotBoughtAt, "1050", "1000"), spotSell(2, soldAt, "1120", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{}).Outcome()

		assert.NotNil(t, outcome.ReturnRate)
		assert.Nil(t, outcome.RMultiple)
		assert.Equal(t, "noStopLoss", outcome.RMultipleUnavailableReason)
	})

	t.Run("the worst and best prices while held and the share captured", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", price("1000"),
			spotBuy(1, spotBoughtAt, "1050", "1000"), spotSell(2, soldAt, "1120", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{
			ExtremesRequested: true,
			PriceExtremes: vo.PriceExtremesVo{
				Has: true, LowestPrice: decimal.RequireFromString("1020"), HighestPrice: decimal.RequireFromString("1150"),
			},
		}).Outcome()

		assert.True(t, outcome.Excursion.Available)
		assert.Equal(t, "1020", outcome.Excursion.AdversePrice.String())
		assert.Equal(t, "1150", outcome.Excursion.FavorablePrice.String())
		assert.Equal(t, "-30000", outcome.Excursion.AdverseProfit.String())
		assert.Equal(t, "100000", outcome.Excursion.FavorableProfit.String())
		require.NotNil(t, outcome.ProfitCaptureRate)
		assert.InDelta(t, 0.7, *outcome.ProfitCaptureRate, 0.0001)
	})

	t.Run("no candles while held leaves the excursions unavailable", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", price("1000"),
			spotBuy(1, spotBoughtAt, "1050", "1000"), spotSell(2, soldAt, "1120", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{ExtremesRequested: true}).Outcome()

		assert.False(t, outcome.Excursion.Available)
		assert.Equal(t, "noMarketData", outcome.Excursion.UnavailableReason)
		assert.Nil(t, outcome.ProfitCaptureRate)
	})

	t.Run("a summary does not work out excursions", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", price("1000"), spotBuy(1, spotBoughtAt, "1050", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{}).Outcome()

		assert.Equal(t, "notComputed", outcome.Excursion.UnavailableReason)
	})

	t.Run("a held trade is marked at the latest price and has no share captured yet", func(t *testing.T) {
		recordDomain := spotRecordOf("open", decimal.NullDecimal{}, spotBuy(1, spotBoughtAt, "1050", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{
			LatestPrice: decimal.RequireFromString("1080"), HasLatestPrice: true, ExtremesRequested: true,
			PriceExtremes: vo.PriceExtremesVo{
				Has: true, LowestPrice: decimal.RequireFromString("1040"), HighestPrice: decimal.RequireFromString("1090"),
			},
		}).Outcome()

		assert.True(t, outcome.FloatingProfit.Available)
		assert.Equal(t, "30000", outcome.FloatingProfit.Amount.String())
		assert.Equal(t, "1080", outcome.FloatingProfit.Price.String())
		assert.Nil(t, outcome.ProfitCaptureRate)
	})

	t.Run("a held trade without a latest price is not marked", func(t *testing.T) {
		recordDomain := spotRecordOf("open", decimal.NullDecimal{}, spotBuy(1, spotBoughtAt, "1050", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{}).Outcome()

		assert.False(t, outcome.FloatingProfit.Available)
		assert.Equal(t, "noLatestPrice", outcome.FloatingProfit.UnavailableReason)
	})

	t.Run("a closed trade is not marked", func(t *testing.T) {
		recordDomain := spotRecordOf("closed", decimal.NullDecimal{},
			spotBuy(1, spotBoughtAt, "1050", "1000"), spotSell(2, soldAt, "1120", "1000"))

		outcome := domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{HasLatestPrice: true}).Outcome()

		assert.Equal(t, "notOpen", outcome.FloatingProfit.UnavailableReason)
	})

	t.Run("a buy dearer than the bot's reference slips against the trade", func(t *testing.T) {
		record := entities.SpotTradeRecord{
			Market: "taiwanStock", Status: "open", SourceReferencePrice: price("1000"),
			Fills: []entities.SpotTradeFill{spotBuy(1, spotBoughtAt, "1005", "1000")},
		}

		outcome := domains.NewSpotTradeOutcomeDomain(domains.NewSpotTradeRecordDomain(record), vo.SpotTradeMarketFactsVo{}).Outcome()

		require.NotNil(t, outcome.EntrySlippagePercentage)
		assert.InDelta(t, 0.5, *outcome.EntrySlippagePercentage, 0.0001)
	})

	t.Run("a trade without a bot round has no slippage", func(t *testing.T) {
		recordDomain := spotRecordOf("open", decimal.NullDecimal{}, spotBuy(1, spotBoughtAt, "1050", "1000"))

		assert.Nil(t, domains.NewSpotTradeOutcomeDomain(recordDomain, vo.SpotTradeMarketFactsVo{}).Outcome().EntrySlippagePercentage)
	})
}

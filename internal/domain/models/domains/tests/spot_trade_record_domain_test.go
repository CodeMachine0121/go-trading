package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var spotBoughtAt = time.Date(2026, 9, 24, 1, 5, 0, 0, time.UTC)

func spotBuy(id uint, filledAt time.Time, price string, quantity string) entities.SpotTradeFill {
	return entities.SpotTradeFill{
		ID: id, Kind: string(vo.SpotTradeFillKindBuy), FilledAt: filledAt,
		Price: decimal.RequireFromString(price), Quantity: decimal.RequireFromString(quantity),
	}
}

func spotSell(id uint, filledAt time.Time, price string, quantity string) entities.SpotTradeFill {
	fill := spotBuy(id, filledAt, price, quantity)
	fill.Kind = string(vo.SpotTradeFillKindSell)

	return fill
}

func openingSpotTrade(t *testing.T, market string, symbol string, firstBuy entities.SpotTradeFill) domains.SpotTradeRecordDomain {
	recordDomain, err := domains.NewOpeningSpotTradeRecordDomain(
		7, symbol, market, dto.SpotTradeRecordWriteDto{}, firstBuy, nil, ledgerNow)
	require.NoError(t, err)

	return recordDomain
}

func TestNewOpeningSpotTradeRecordDomain(t *testing.T) {
	t.Run("a Taiwan stock buy opens a holding counted in New Taiwan dollars", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(0, spotBoughtAt, "1050", "1000"))

		entity := recordDomain.ToEntity()
		assert.Equal(t, "open", entity.Status)
		assert.Equal(t, "taiwanStock", entity.Market)
		assert.Equal(t, uint(7), entity.OwnerID)
		assert.Equal(t, "TWD", recordDomain.MarketDomain().Currency())
		assert.True(t, entity.OpenedAt.Equal(spotBoughtAt))
	})

	t.Run("a crypto buy counts in USDT and an unknown market reads as crypto", func(t *testing.T) {
		cryptoTrade := openingSpotTrade(t, "crypto", "BTCUSDT", spotBuy(0, spotBoughtAt, "97900", "0.05"))
		legacyTrade := openingSpotTrade(t, "", "BTCUSDT", spotBuy(0, spotBoughtAt, "97900", "0.05"))

		assert.Equal(t, "USDT", cryptoTrade.MarketDomain().Currency())
		assert.Equal(t, "crypto", legacyTrade.ToEntity().Market)
	})

	t.Run("a first fill without a kind is the buy it can only be", func(t *testing.T) {
		unlabelled := spotBuy(0, spotBoughtAt, "1050", "1000")
		unlabelled.Kind = ""

		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", unlabelled)

		assert.Equal(t, "buy", recordDomain.ToEntity().Fills[0].Kind)
	})

	testCases := []struct {
		name            string
		writeDto        dto.SpotTradeRecordWriteDto
		market          string
		firstFill       entities.SpotTradeFill
		expectedMessage string
	}{
		{name: "a leverage", writeDto: dto.SpotTradeRecordWriteDto{Leverage: price("3")}, market: "crypto",
			firstFill: spotBuy(0, spotBoughtAt, "97900", "0.05"), expectedMessage: "現貨只有先買後賣，沒有槓桿"},
		{name: "a direction", writeDto: dto.SpotTradeRecordWriteDto{Direction: "short"}, market: "crypto",
			firstFill: spotBuy(0, spotBoughtAt, "97900", "0.05"), expectedMessage: "現貨只有先買後賣，沒有槓桿"},
		{name: "selling first", market: "crypto", firstFill: spotSell(0, spotBoughtAt, "97900", "0.05"),
			expectedMessage: "現貨只有先買後賣，第一筆必須是買進"},
		{name: "part of a Taiwan share", market: "taiwanStock", firstFill: spotBuy(0, spotBoughtAt, "1050", "1000.5"),
			expectedMessage: "台股數量以股計，必須是整數"},
		{name: "a zero price", market: "crypto", firstFill: spotBuy(0, spotBoughtAt, "0", "1"),
			expectedMessage: "價格與數量必須大於零"},
		{name: "a stop above the buy", writeDto: dto.SpotTradeRecordWriteDto{
			Plan: dto.SpotTradePlanWriteDto{PlannedStopLossPrice: price("1080")}}, market: "taiwanStock",
			firstFill: spotBuy(0, spotBoughtAt, "1050", "1000"), expectedMessage: "止損必須低於買進價"},
		{name: "a target below the buy", writeDto: dto.SpotTradeRecordWriteDto{
			Plan: dto.SpotTradePlanWriteDto{PlannedTakeProfitPrice: price("1000")}}, market: "taiwanStock",
			firstFill: spotBuy(0, spotBoughtAt, "1050", "1000"), expectedMessage: "止盈必須高於買進價"},
		{name: "a negative stop", writeDto: dto.SpotTradeRecordWriteDto{
			Plan: dto.SpotTradePlanWriteDto{PlannedStopLossPrice: price("-1")}}, market: "taiwanStock",
			firstFill: spotBuy(0, spotBoughtAt, "1050", "1000"), expectedMessage: "計畫止損不得為負"},
		{name: "a confidence of six", writeDto: dto.SpotTradeRecordWriteDto{
			Plan: dto.SpotTradePlanWriteDto{Confidence: confidenceOf(6)}}, market: "taiwanStock",
			firstFill: spotBuy(0, spotBoughtAt, "1050", "1000"), expectedMessage: "信心只能是 1 到 5"},
		{name: "a mistake tag as a setup tag", market: "crypto",
			firstFill: spotBuy(0, spotBoughtAt, "97900", "0.05"), expectedMessage: "不能用在這裡"},
	}

	for _, testCase := range testCases {
		t.Run("refuses "+testCase.name, func(t *testing.T) {
			setupTags := []entities.TradeTag{}
			if testCase.name == "a mistake tag as a setup tag" {
				setupTags = []entities.TradeTag{{ID: 1, Kind: "mistake", Name: "追價進場"}}
			}

			_, err := domains.NewOpeningSpotTradeRecordDomain(
				7, "SYMBOL", testCase.market, testCase.writeDto, testCase.firstFill, setupTags, ledgerNow)

			require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestSpotTradeRecordDomainBuysAndSells(t *testing.T) {
	t.Run("selling part keeps the trade held", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(0, spotBoughtAt, "1050", "1000"))

		require.NoError(t, recordDomain.AddFill(spotSell(0, spotBoughtAt.Add(time.Hour), "1100", "400"), ledgerNow))

		assert.Equal(t, "600", recordDomain.Ledger().Position().String())
		assert.True(t, recordDomain.IsOpen())
	})

	t.Run("selling everything closes it, dated by the sale", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(0, spotBoughtAt, "1050", "600"))
		soldAt := spotBoughtAt.Add(26 * time.Hour)

		require.NoError(t, recordDomain.AddFill(spotSell(0, soldAt, "1120", "600"), ledgerNow))

		entity := recordDomain.ToEntity()
		assert.Equal(t, "closed", entity.Status)
		assert.True(t, entity.ClosedAt.Equal(soldAt))
	})

	t.Run("selling more than is held is refused", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(0, spotBoughtAt, "1050", "600"))

		err := recordDomain.AddFill(spotSell(0, spotBoughtAt.Add(time.Hour), "1120", "800"), ledgerNow)

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "賣出數量超過目前持有 600")
		assert.Equal(t, "600", recordDomain.Ledger().Position().String())
	})

	t.Run("a sale of part of a Taiwan share is refused", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(0, spotBoughtAt, "1050", "600"))

		err := recordDomain.AddFill(spotSell(0, spotBoughtAt.Add(time.Hour), "1120", "0.5"), ledgerNow)

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "台股數量以股計，必須是整數")
	})

	t.Run("a mistyped buy is corrected while held and a spare one removed", func(t *testing.T) {
		recordDomain := domains.NewSpotTradeRecordDomain(entities.SpotTradeRecord{
			ID: 5, Market: "taiwanStock", Status: "open",
			Fills: []entities.SpotTradeFill{spotBuy(1, spotBoughtAt, "1005", "1000"), spotBuy(2, spotBoughtAt, "1060", "100")},
		})

		require.NoError(t, recordDomain.AmendFill(1, spotBuy(0, spotBoughtAt, "1050", "1000"), ledgerNow))
		require.NoError(t, recordDomain.RemoveFill(2, ledgerNow))

		entity := recordDomain.ToEntity()
		assert.Equal(t, uint(1), entity.Fills[0].ID)
		assert.Equal(t, uint(5), entity.Fills[0].SpotTradeRecordID)
		assert.Equal(t, "1050", recordDomain.Ledger().AverageEntryPrice().String())
	})

	t.Run("a buy or sell the trade does not have cannot be changed", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))

		amendError := recordDomain.AmendFill(9, spotBuy(0, spotBoughtAt, "1", "1"), ledgerNow)
		removeError := recordDomain.RemoveFill(9, ledgerNow)

		require.ErrorIs(t, amendError, domains.ErrSpotTradeValidation)
		require.ErrorIs(t, removeError, domains.ErrSpotTradeValidation)
		assert.Contains(t, amendError.Error(), "這筆交易沒有識別碼為 9 的買賣")
	})

	t.Run("the only buy cannot be removed", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))

		err := recordDomain.RemoveFill(1, ledgerNow)

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "一筆交易至少要有一筆買進")
	})
}

func closedSpotTrade(t *testing.T) domains.SpotTradeRecordDomain {
	recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))
	require.NoError(t, recordDomain.AddFill(spotSell(0, spotBoughtAt.Add(time.Hour), "1120", "600"), ledgerNow))

	return recordDomain
}

func TestSpotTradeRecordDomainLocksAClosedTrade(t *testing.T) {
	testCases := []struct {
		name            string
		change          func(recordDomain *domains.SpotTradeRecordDomain) error
		expectedMessage string
	}{
		{name: "adding a buy", expectedMessage: "這筆交易已經平倉，不能再加買進或賣出",
			change: func(recordDomain *domains.SpotTradeRecordDomain) error {
				return recordDomain.AddFill(spotBuy(0, spotBoughtAt.Add(2*time.Hour), "1100", "1"), ledgerNow)
			}},
		{name: "amending a buy", expectedMessage: "平倉後買賣已鎖定，可以加附註或刪除整筆重記",
			change: func(recordDomain *domains.SpotTradeRecordDomain) error {
				return recordDomain.AmendFill(1, spotBuy(0, spotBoughtAt, "1000", "600"), ledgerNow)
			}},
		{name: "removing a buy", expectedMessage: "平倉後買賣已鎖定，可以加附註或刪除整筆重記",
			change: func(recordDomain *domains.SpotTradeRecordDomain) error {
				return recordDomain.RemoveFill(1, ledgerNow)
			}},
		{name: "changing the plan", expectedMessage: "平倉後計畫已鎖定，可以加附註",
			change: func(recordDomain *domains.SpotTradeRecordDomain) error {
				return recordDomain.AmendPlan(dto.SpotTradePlanWriteDto{PlannedStopLossPrice: price("1000")})
			}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recordDomain := closedSpotTrade(t)

			err := testCase.change(&recordDomain)

			require.ErrorIs(t, err, domains.ErrSpotTradeLocked)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}

	t.Run("a note leaves the plan as it was", func(t *testing.T) {
		recordDomain := closedSpotTrade(t)

		require.NoError(t, recordDomain.AddNote("  止損其實是 1,000  ", ledgerNow))
		blankError := recordDomain.AddNote("   ", ledgerNow)

		entity := recordDomain.ToEntity()
		assert.Equal(t, "止損其實是 1,000", entity.Notes[0].Content)
		require.ErrorIs(t, blankError, domains.ErrSpotTradeValidation)
		assert.Contains(t, blankError.Error(), "附註不得為空白")
	})
}

func TestSpotTradeRecordDomainPlansReviewsAndTags(t *testing.T) {
	t.Run("the plan changes while held", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))

		require.NoError(t, recordDomain.AmendPlan(dto.SpotTradePlanWriteDto{
			PlannedStopLossPrice: price("1000"), PlannedTakeProfitPrice: price("1150"),
			EntryReason: "  站上季線  ", Confidence: confidenceOf(4),
		}))

		entity := recordDomain.ToEntity()
		assert.Equal(t, "1000", entity.PlannedStopLossPrice.Decimal.String())
		assert.Equal(t, "站上季線", entity.EntryReason)
		assert.Equal(t, 4, *entity.Confidence)
	})

	t.Run("a held trade cannot be reviewed", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))

		err := recordDomain.WriteReview(dto.SpotTradeReviewWriteDto{ExecutionScore: 4}, nil, ledgerNow)

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "平倉後才能檢討")
	})

	t.Run("a closed trade is reviewed with shared mistake tags", func(t *testing.T) {
		recordDomain := closedSpotTrade(t)
		chasing := entities.TradeTag{ID: 3, Kind: "mistake", Name: "追價進場"}

		require.NoError(t, recordDomain.WriteReview(dto.SpotTradeReviewWriteDto{
			WentWell: " 停損有守 ", ExecutionScore: 4}, []entities.TradeTag{chasing}, ledgerNow))

		entity := recordDomain.ToEntity()
		assert.Equal(t, "reviewed", entity.Status)
		assert.Equal(t, "停損有守", entity.ReviewWentWell)
		assert.Equal(t, []entities.TradeTag{chasing}, entity.Tags)
	})

	t.Run("an execution score of six is refused", func(t *testing.T) {
		recordDomain := closedSpotTrade(t)

		err := recordDomain.WriteReview(dto.SpotTradeReviewWriteDto{ExecutionScore: 6}, nil, ledgerNow)

		require.ErrorIs(t, err, domains.ErrSpotTradeValidation)
		assert.Contains(t, err.Error(), "執行評分只能是 1 到 5")
	})

	t.Run("setup tags replace setup tags only", func(t *testing.T) {
		recordDomain := closedSpotTrade(t)
		mistake := entities.TradeTag{ID: 3, Kind: "mistake", Name: "追價進場"}
		breakout := entities.TradeTag{ID: 4, Kind: "setup", Name: "突破"}
		require.NoError(t, recordDomain.WriteReview(dto.SpotTradeReviewWriteDto{ExecutionScore: 3},
			[]entities.TradeTag{mistake}, ledgerNow))

		require.NoError(t, recordDomain.AssignSetupTags([]entities.TradeTag{breakout}))

		assert.ElementsMatch(t, []entities.TradeTag{mistake, breakout}, recordDomain.ToEntity().Tags)
	})

	t.Run("a bot round's suggestion is copied onto the trade", func(t *testing.T) {
		recordDomain := openingSpotTrade(t, "taiwanStock", "2330", spotBuy(1, spotBoughtAt, "1050", "600"))

		recordDomain.WithSource(dto.JournalLinkRoundDto{
			StrategyBotID: 3, StrategyBotName: "台積電波段", RunNumber: 88,
			ReferencePrice: price("1045"), SuggestedStopLossPrice: price("1000"), SuggestedTakeProfitPrice: price("1150"),
		})

		entity := recordDomain.ToEntity()
		assert.Equal(t, uint(3), *entity.SourceStrategyBotID)
		assert.Equal(t, 88, *entity.SourceRunNumber)
		assert.Equal(t, "1045", entity.SourceReferencePrice.Decimal.String())
		assert.Equal(t, "台積電波段", entity.SourceStrategyBotName)
	})
}

func TestSpotTradeMarketDomain(t *testing.T) {
	testCases := []struct {
		name             string
		market           string
		stake            string
		price            string
		expectedQuantity string
	}{
		{name: "Taiwan stock is floored to whole shares", market: "taiwanStock", stake: "105000", price: "1050", expectedQuantity: "100"},
		{name: "a Taiwan stake short of the next share", market: "taiwanStock", stake: "105999", price: "1050", expectedQuantity: "100"},
		{name: "crypto is floored to eight decimals", market: "crypto", stake: "1000", price: "97900", expectedQuantity: "0.01021450"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			quantity := domains.NewSpotTradeMarketDomain(testCase.market).PrefillQuantityOf(
				decimal.RequireFromString(testCase.stake), decimal.RequireFromString(testCase.price))

			assert.True(t, quantity.Equal(decimal.RequireFromString(testCase.expectedQuantity)), quantity.String())
		})
	}

	t.Run("whole shares pass and crypto fractions pass", func(t *testing.T) {
		assert.NoError(t, domains.NewSpotTradeMarketDomain("taiwanStock").RequireQuantity(decimal.RequireFromString("1000")))
		assert.NoError(t, domains.NewSpotTradeMarketDomain("crypto").RequireQuantity(decimal.RequireFromString("0.05")))
	})
}

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

func feeRatesOf(makerFeeRate string, takerFeeRate string) domains.TradeJournalSettingDomain {
	return domains.NewTradeJournalSettingDomain(entities.TradeJournalSetting{
		MakerFeeRate: price(makerFeeRate), TakerFeeRate: price(takerFeeRate),
	})
}

func TestTradeJournalSettingDomainPricesFills(t *testing.T) {
	t.Run("a taker fill pays the taker rate", func(t *testing.T) {
		fee, rateMissing := feeRatesOf("0.02", "0.05").FeeFor(
			decimal.RequireFromString("97905"), decimal.RequireFromString("0.030"), vo.TradeFillLiquidityTaker)

		assert.Equal(t, "1.47", fee.StringFixed(2))
		assert.False(t, rateMissing)
	})

	t.Run("a maker fill pays the maker rate", func(t *testing.T) {
		fee, _ := feeRatesOf("0.02", "0.05").FeeFor(
			decimal.RequireFromString("100000"), decimal.RequireFromString("1"), vo.TradeFillLiquidityMaker)

		assert.Equal(t, "20", fee.String())
	})

	t.Run("no rate set is a zero fee marked as missing", func(t *testing.T) {
		fee, rateMissing := domains.NewTradeJournalSettingDomain(entities.TradeJournalSetting{}).FeeFor(
			decimal.RequireFromString("97905"), decimal.RequireFromString("0.030"), vo.TradeFillLiquidityTaker)

		assert.True(t, fee.IsZero())
		assert.True(t, rateMissing)
	})

	t.Run("a blank fill takes now, taker and the rate's fee", func(t *testing.T) {
		fill := feeRatesOf("0.02", "0.05").PricedFill(dto.ContractTradeFillWriteDto{
			Kind: "entry", Price: decimal.RequireFromString("97905"), Quantity: decimal.RequireFromString("0.030"),
		}, ledgerNow)

		assert.True(t, fill.FilledAt.Equal(ledgerNow))
		assert.Equal(t, string(vo.TradeFillLiquidityTaker), fill.Liquidity)
		assert.Equal(t, "1.47", fill.Fee.StringFixed(2))
	})

	t.Run("a written fee and time win", func(t *testing.T) {
		filledAt := time.Date(2026, 9, 25, 14, 3, 0, 0, time.UTC)

		fill := feeRatesOf("0.02", "0.05").PricedFill(dto.ContractTradeFillWriteDto{
			Kind: "entry", FilledAt: &filledAt, Liquidity: "maker",
			Price: decimal.RequireFromString("97905"), Quantity: decimal.RequireFromString("0.030"),
			Fee: price("1.20"),
		}, ledgerNow)

		assert.True(t, fill.FilledAt.Equal(filledAt))
		assert.Equal(t, "1.2", fill.Fee.String())
		assert.False(t, fill.FeeRateMissing)
	})
}

func TestTradeJournalSettingDomainFeeRates(t *testing.T) {
	t.Run("rates are kept and a rate change never reaches the seeding marker", func(t *testing.T) {
		seededAt := ledgerNow
		setting := domains.NewTradeJournalSettingDomain(entities.TradeJournalSetting{DefaultMistakeTagsSeededAt: &seededAt})

		updated, err := setting.WithFeeRates(dto.TradeJournalSettingWriteDto{
			MakerFeeRate: price("0.02"), TakerFeeRate: price("0.05")})

		require.NoError(t, err)
		entity := updated.ToEntity(7)
		assert.Equal(t, uint(7), entity.UserID)
		assert.Equal(t, "0.05", updated.TakerFeeRate().String())
		assert.False(t, updated.NeedsDefaultMistakeTags())
	})

	t.Run("a negative rate is refused", func(t *testing.T) {
		_, err := domains.NewTradeJournalSettingDomain(entities.TradeJournalSetting{}).WithFeeRates(
			dto.TradeJournalSettingWriteDto{TakerFeeRate: price("-0.01")})

		require.ErrorIs(t, err, domains.ErrTradeJournalSettingValidation)
		assert.Contains(t, err.Error(), "手續費率不得為負")
	})

	t.Run("a person who never had tags needs the defaults", func(t *testing.T) {
		assert.True(t, domains.NewTradeJournalSettingDomain(entities.TradeJournalSetting{}).NeedsDefaultMistakeTags())
		assert.Equal(t, []string{"追價進場", "移動止損", "提早出場", "部位過大", "報復性交易"}, domains.DefaultMistakeTagNames)
	})
}

func TestNewTradeTagDomain(t *testing.T) {
	t.Run("a setup tag keeps its trimmed name", func(t *testing.T) {
		tagDomain, err := domains.NewTradeTagDomain(dto.TradeTagWriteDto{Kind: "setup", Name: "  突破 "})

		require.NoError(t, err)
		assert.Equal(t, "突破", tagDomain.Name())
		assert.Equal(t, entities.TradeTag{OwnerID: 7, Kind: "setup", Name: "突破"}, tagDomain.ToEntity(7))
	})

	testCases := []struct {
		name            string
		writeDto        dto.TradeTagWriteDto
		expectedMessage string
	}{
		{name: "an unknown kind", writeDto: dto.TradeTagWriteDto{Kind: "mood", Name: "急"},
			expectedMessage: "標籤種類只有失誤（mistake）與型態（setup）"},
		{name: "a blank name", writeDto: dto.TradeTagWriteDto{Kind: "mistake", Name: "  "},
			expectedMessage: "標籤名稱不得為空白"},
		{name: "a name longer than sixty-four", writeDto: dto.TradeTagWriteDto{Kind: "mistake",
			Name: "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五"},
			expectedMessage: "標籤名稱不得超過 64 個字"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewTradeTagDomain(testCase.writeDto)

			require.ErrorIs(t, err, domains.ErrTradeTagValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestTradeStatisticsPeriodDomain(t *testing.T) {
	testCases := []struct {
		period        string
		expectedValue string
		expectedSince *time.Time
	}{
		{period: "", expectedValue: "30d", expectedSince: new(ledgerNow.AddDate(0, 0, -30))},
		{period: "7d", expectedValue: "7d", expectedSince: new(ledgerNow.AddDate(0, 0, -7))},
		{period: "90d", expectedValue: "90d", expectedSince: new(ledgerNow.AddDate(0, 0, -90))},
		{period: "all", expectedValue: "all", expectedSince: nil},
	}

	for _, testCase := range testCases {
		t.Run("period "+testCase.expectedValue, func(t *testing.T) {
			periodDomain, err := domains.NewTradeStatisticsPeriodDomain(testCase.period, domains.ErrContractTradeValidation)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedValue, periodDomain.Value())
			assert.Equal(t, testCase.expectedSince, periodDomain.Since(ledgerNow))
		})
	}

	t.Run("an unknown period is refused", func(t *testing.T) {
		_, err := domains.NewTradeStatisticsPeriodDomain("1y", domains.ErrContractTradeValidation)

		require.ErrorIs(t, err, domains.ErrContractTradeValidation)
		assert.Contains(t, err.Error(), "期間只有 7d、30d、90d 與 all")
	})
}

func TestFundingSettlementScheduleDomain(t *testing.T) {
	fourHours := 4
	eightHourSchedule := domains.NewFundingSettlementScheduleDomain(entities.ContractTradingSymbol{})
	fourHourSchedule := domains.NewFundingSettlementScheduleDomain(
		entities.ContractTradingSymbol{FundingIntervalHours: &fourHours})
	morning := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)

	assert.False(t, eightHourSchedule.IsDueBetween(morning, morning.Add(6*time.Hour)))
	assert.True(t, eightHourSchedule.IsDueBetween(morning, morning.Add(7*time.Hour)))
	assert.True(t, fourHourSchedule.IsDueBetween(morning, morning.Add(3*time.Hour)))
}

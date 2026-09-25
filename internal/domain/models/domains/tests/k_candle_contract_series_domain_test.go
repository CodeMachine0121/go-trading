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

func seriesMinute(minute int) time.Time {
	return time.Date(2026, 9, 23, 9, minute, 0, 0, time.UTC)
}

func present(value string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(value))
}

// minuteOfContract derives every figure from one level so a merge picking the wrong candle shows the wrong number.
func minuteOfContract(minute int, level int64) entities.KCandleContract {
	price := decimal.NewFromInt(level)

	return entities.KCandleContract{
		Symbol: "BTCUSDT", OpenTime: seriesMinute(minute),
		Open: price, High: price.Add(decimal.NewFromInt(1)), Low: price.Sub(decimal.NewFromInt(1)), Close: price,
		Volume: decimal.NewFromInt(2), QuoteVolume: decimal.NewFromInt(200),
		TakerBuyBaseVolume: decimal.NewFromInt(1), TakerBuyQuoteVolume: decimal.NewFromInt(100),
		TradeCount: 7,
		MarkOpen:   price, MarkHigh: price.Add(decimal.NewFromInt(2)), MarkLow: price.Sub(decimal.NewFromInt(2)), MarkClose: price,
		IndexOpen: decimal.NewNullDecimal(price), IndexHigh: decimal.NewNullDecimal(price.Add(decimal.NewFromInt(3))),
		IndexLow: decimal.NewNullDecimal(price.Sub(decimal.NewFromInt(3))), IndexClose: decimal.NewNullDecimal(price),
		PremiumIndexOpen: present("-0.0001"), PremiumIndexHigh: present("0.0002"),
		PremiumIndexLow: present("-0.0003"), PremiumIndexClose: present("0.0001"),
	}
}

func fiveMinutes(t *testing.T) domains.AggregationIntervalDomain {
	t.Helper()

	interval, intervalError := domains.NewAggregationIntervalDomain("5m")
	require.NoError(t, intervalError)

	return interval
}

func TestKCandleContractSeriesDomainMergesABucketIntoOneCandle(t *testing.T) {
	// Handed over out of order: the merge reads open time, not arrival.
	kCandles := []entities.KCandleContract{
		minuteOfContract(2, 110), minuteOfContract(0, 100), minuteOfContract(4, 104),
		minuteOfContract(1, 120), minuteOfContract(3, 90),
	}
	kCandles[1].PremiumIndexLow = present("-0.0009")
	kCandles[3].MarkHigh = decimal.RequireFromString("125")

	series := domains.NewKCandleContractSeriesDomain("BTCUSDT", fiveMinutes(t), kCandles).ToDto()

	assert.Equal(t, "BTCUSDT", series.Symbol)
	assert.Equal(t, "5m", series.Interval)
	require.Len(t, series.KCandles, 1)
	merged := series.KCandles[0]
	assert.Equal(t, seriesMinute(0), merged.OpenTime)
	assert.True(t, decimal.RequireFromString("100").Equal(merged.Open), "開盤取最早那根")
	assert.True(t, decimal.RequireFromString("104").Equal(merged.Close), "收盤取最晚那根")
	assert.True(t, decimal.RequireFromString("121").Equal(merged.High))
	assert.True(t, decimal.RequireFromString("89").Equal(merged.Low))
	assert.True(t, decimal.RequireFromString("10").Equal(merged.Volume))
	assert.True(t, decimal.RequireFromString("1000").Equal(merged.QuoteVolume))
	assert.True(t, decimal.RequireFromString("5").Equal(merged.TakerBuyBaseVolume))
	assert.True(t, decimal.RequireFromString("500").Equal(merged.TakerBuyQuoteVolume))
	assert.Equal(t, int64(35), merged.TradeCount)
	assert.True(t, decimal.RequireFromString("100").Equal(merged.MarkOpen))
	assert.True(t, decimal.RequireFromString("125").Equal(merged.MarkHigh))
	assert.True(t, decimal.RequireFromString("88").Equal(merged.MarkLow))
	assert.True(t, decimal.RequireFromString("104").Equal(merged.MarkClose))
	assert.True(t, decimal.RequireFromString("100").Equal(merged.IndexOpen.Decimal))
	assert.True(t, decimal.RequireFromString("123").Equal(merged.IndexHigh.Decimal))
	assert.True(t, decimal.RequireFromString("87").Equal(merged.IndexLow.Decimal))
	assert.True(t, decimal.RequireFromString("104").Equal(merged.IndexClose.Decimal))
	assert.True(t, decimal.RequireFromString("-0.0001").Equal(merged.PremiumIndexOpen.Decimal))
	assert.True(t, decimal.RequireFromString("-0.0009").Equal(merged.PremiumIndexLow.Decimal))
	assert.True(t, decimal.RequireFromString("0.0002").Equal(merged.PremiumIndexHigh.Decimal))
	assert.True(t, decimal.RequireFromString("0.0001").Equal(merged.PremiumIndexClose.Decimal))
}

func TestKCandleContractSeriesDomainLeavesALineOutOfABucketThatLacksItAnywhere(t *testing.T) {
	testCases := []struct {
		name        string
		lackingAt   int
		lackIndex   bool
		lackPremium bool
	}{
		{name: "第一根沒有指數價格", lackingAt: 0, lackIndex: true},
		{name: "中間一根沒有指數價格", lackingAt: 2, lackIndex: true},
		{name: "最後一根沒有溢價指數", lackingAt: 4, lackPremium: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			kCandles := []entities.KCandleContract{}
			for minute := range 5 {
				kCandles = append(kCandles, minuteOfContract(minute, 100))
			}
			if testCase.lackIndex {
				kCandles[testCase.lackingAt].IndexOpen = decimal.NullDecimal{}
			}
			if testCase.lackPremium {
				kCandles[testCase.lackingAt].PremiumIndexClose = decimal.NullDecimal{}
			}

			merged := domains.NewKCandleContractSeriesDomain("BTCUSDT", fiveMinutes(t), kCandles).ToDto().KCandles[0]

			assert.Equal(t, !testCase.lackIndex, merged.IndexOpen.Valid)
			assert.Equal(t, !testCase.lackIndex, merged.IndexHigh.Valid)
			assert.Equal(t, !testCase.lackIndex, merged.IndexLow.Valid)
			assert.Equal(t, !testCase.lackIndex, merged.IndexClose.Valid)
			assert.Equal(t, !testCase.lackPremium, merged.PremiumIndexOpen.Valid)
			assert.Equal(t, !testCase.lackPremium, merged.PremiumIndexClose.Valid)
			assert.True(t, decimal.RequireFromString("100").Equal(merged.MarkOpen), "標記價格照常合併")
		})
	}
}

func TestKCandleContractSeriesDomainProducesNothingForAnEmptyBucket(t *testing.T) {
	kCandles := []entities.KCandleContract{
		minuteOfContract(0, 100), minuteOfContract(11, 110), minuteOfContract(1, 101),
	}

	series := domains.NewKCandleContractSeriesDomain("BTCUSDT", fiveMinutes(t), kCandles).ToDto()

	require.Len(t, series.KCandles, 2)
	assert.Equal(t, seriesMinute(0), series.KCandles[0].OpenTime)
	assert.Equal(t, seriesMinute(10), series.KCandles[1].OpenTime)
	assert.Equal(t, int64(14), series.KCandles[0].TradeCount)
}

func TestKCandleContractSeriesDomainOfNothingIsAnEmptySeries(t *testing.T) {
	series := domains.NewKCandleContractSeriesDomain("BTCUSDT", fiveMinutes(t), nil).ToDto()

	assert.Empty(t, series.KCandles)
	assert.NotNil(t, series.KCandles)
}

func TestKCandleSeriesQueryDomainCutsContractCandlesIntoTheBucketsItChose(t *testing.T) {
	roundTheClock := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}).
		MarketOf(string(vo.MarketCrypto))
	displayable := 100
	seriesQuery, queryError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol: "BTCUSDT", StartTime: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
		EndTime: time.Date(2026, 9, 23, 23, 59, 0, 0, time.UTC), DisplayableCandleCount: &displayable,
	}, roundTheClock, 1000)
	require.NoError(t, queryError)

	series := seriesQuery.ContractSeriesOf([]entities.KCandleContract{
		minuteOfContract(0, 100), minuteOfContract(20, 120),
	}).ToDto()

	assert.Equal(t, "15m", series.Interval)
	require.Len(t, series.KCandles, 2)
	assert.Equal(t, time.Date(2026, 9, 23, 9, 15, 0, 0, time.UTC), series.KCandles[1].OpenTime)
}

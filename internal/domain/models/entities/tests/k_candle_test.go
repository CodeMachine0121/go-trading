package entities_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestKCandleToDto(t *testing.T) {
	openTime := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	testCases := []struct {
		name    string
		kCandle entities.KCandle
	}{
		{
			name: "carries the symbol, the open time and every price and volume figure",
			kCandle: entities.KCandle{
				Symbol:              "BTCUSDT",
				OpenTime:            openTime,
				Open:                decimal.RequireFromString("100.1"),
				High:                decimal.RequireFromString("120.2"),
				Low:                 decimal.RequireFromString("90.3"),
				Close:               decimal.RequireFromString("110.4"),
				Volume:              decimal.RequireFromString("11.5"),
				QuoteVolume:         decimal.RequireFromString("1200.6"),
				TakerBuyBaseVolume:  decimal.RequireFromString("5.7"),
				TakerBuyQuoteVolume: decimal.RequireFromString("600.8"),
			},
		},
		{
			name: "keeps the smallest storable fraction without losing precision",
			kCandle: entities.KCandle{
				Symbol:              "ETHUSDT",
				OpenTime:            openTime,
				Open:                decimal.RequireFromString("0.000000000000000001"),
				High:                decimal.RequireFromString("0.000000000000000002"),
				Low:                 decimal.RequireFromString("0.000000000000000001"),
				Close:               decimal.RequireFromString("0.000000000000000002"),
				Volume:              decimal.RequireFromString("0.000000000000000003"),
				QuoteVolume:         decimal.RequireFromString("0.000000000000000004"),
				TakerBuyBaseVolume:  decimal.RequireFromString("0.000000000000000005"),
				TakerBuyQuoteVolume: decimal.RequireFromString("0.000000000000000006"),
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			kCandleDto := testCase.kCandle.ToDto()

			assert.Equal(t, testCase.kCandle.Symbol, kCandleDto.Symbol)
			assert.Equal(t, testCase.kCandle.OpenTime, kCandleDto.OpenTime)
			assert.True(t, testCase.kCandle.Open.Equal(kCandleDto.Open))
			assert.True(t, testCase.kCandle.High.Equal(kCandleDto.High))
			assert.True(t, testCase.kCandle.Low.Equal(kCandleDto.Low))
			assert.True(t, testCase.kCandle.Close.Equal(kCandleDto.Close))
			assert.True(t, testCase.kCandle.Volume.Equal(kCandleDto.Volume))
			assert.True(t, testCase.kCandle.QuoteVolume.Equal(kCandleDto.QuoteVolume))
			assert.True(t, testCase.kCandle.TakerBuyBaseVolume.Equal(kCandleDto.TakerBuyBaseVolume))
			assert.True(t, testCase.kCandle.TakerBuyQuoteVolume.Equal(kCandleDto.TakerBuyQuoteVolume))
		})
	}
}

func TestKCandleToDtoHandsOutTheOpenTimeInUniversalTime(t *testing.T) {
	elsewhere := time.FixedZone("UTC+8", 8*60*60)
	openTimeElsewhere := time.Date(2026, 8, 29, 17, 0, 0, 0, elsewhere)

	kCandleDto := entities.KCandle{Symbol: "BTCUSDT", OpenTime: openTimeElsewhere}.ToDto()

	assert.Equal(t, time.UTC, kCandleDto.OpenTime.Location())
	assert.Equal(t, "2026-08-29T09:00:00Z", kCandleDto.OpenTime.Format(time.RFC3339))
	assert.True(t, openTimeElsewhere.Equal(kCandleDto.OpenTime))
}

func TestKCandleToVo(t *testing.T) {
	openTime := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	kCandleVo := entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100.5"),
		High:                decimal.RequireFromString("120.25"),
		Low:                 decimal.RequireFromString("90.75"),
		Close:               decimal.RequireFromString("110.125"),
		Volume:              decimal.RequireFromString("11.5"),
		QuoteVolume:         decimal.RequireFromString("1200.5"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5.25"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600.75"),
	}.ToVo()

	assert.Equal(t, "BTCUSDT", kCandleVo.Symbol)
	assert.Equal(t, openTime.Unix(), kCandleVo.OpenTimeUnixSeconds)
	assert.Equal(t, 100.5, kCandleVo.Open)
	assert.Equal(t, 120.25, kCandleVo.High)
	assert.Equal(t, 90.75, kCandleVo.Low)
	assert.Equal(t, 110.125, kCandleVo.Close)
	assert.Equal(t, 11.5, kCandleVo.Volume)
	assert.Equal(t, 1200.5, kCandleVo.QuoteVolume)
	assert.Equal(t, 5.25, kCandleVo.TakerBuyBaseVolume)
	assert.Equal(t, 600.75, kCandleVo.TakerBuyQuoteVolume)
}

func TestKCandleToVoCarriesTheOpenTimeAsUniversalSeconds(t *testing.T) {
	elsewhere := time.FixedZone("UTC+8", 8*60*60)

	kCandleVo := entities.KCandle{
		OpenTime: time.Date(2026, 8, 29, 17, 0, 0, 0, elsewhere),
	}.ToVo()

	assert.Equal(t, time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC).Unix(), kCandleVo.OpenTimeUnixSeconds)
}

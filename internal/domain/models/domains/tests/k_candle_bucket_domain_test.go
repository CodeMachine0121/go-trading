package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// buildBucketKCandle names only the figures a merging rule reads; everything else is
// filled with a value that cannot be mistaken for one under test.
func buildBucketKCandle(
	t *testing.T, openTime string, open string, high string, low string, closePrice string, volume string,
) entities.KCandle {
	t.Helper()

	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            mustParseTime(t, openTime),
		Open:                decimal.RequireFromString(open),
		High:                decimal.RequireFromString(high),
		Low:                 decimal.RequireFromString(low),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString(volume),
		QuoteVolume:         decimal.RequireFromString(volume).Mul(decimal.NewFromInt(10)),
		TakerBuyBaseVolume:  decimal.RequireFromString(volume).Div(decimal.NewFromInt(2)),
		TakerBuyQuoteVolume: decimal.RequireFromString(volume).Mul(decimal.NewFromInt(5)),
	}
}

func TestKCandleBucketDomainMergesTheCandlesItHolds(t *testing.T) {
	bucketStart := mustParseTime(t, "2026-09-02T10:00:00Z")
	bucketDomain := domains.NewKCandleBucketDomain(bucketStart, []entities.KCandle{
		buildBucketKCandle(t, "2026-09-02T10:00:00Z", "100", "130", "95", "120", "3"),
		buildBucketKCandle(t, "2026-09-02T10:05:00Z", "120", "140", "90", "110", "7"),
	})

	mergedKCandle := bucketDomain.ToDto()

	assert.Equal(t, "BTCUSDT", mergedKCandle.Symbol)
	assert.Equal(t, bucketStart, mergedKCandle.OpenTime)
	assert.True(t, decimal.RequireFromString("100").Equal(mergedKCandle.Open), "open comes from the earliest candle")
	assert.True(t, decimal.RequireFromString("140").Equal(mergedKCandle.High), "high is the highest high")
	assert.True(t, decimal.RequireFromString("90").Equal(mergedKCandle.Low), "low is the lowest low")
	assert.True(t, decimal.RequireFromString("110").Equal(mergedKCandle.Close), "close comes from the latest candle")
	assert.True(t, decimal.RequireFromString("10").Equal(mergedKCandle.Volume), "volume is the sum")
	assert.True(t, decimal.RequireFromString("100").Equal(mergedKCandle.QuoteVolume))
	assert.True(t, decimal.RequireFromString("5").Equal(mergedKCandle.TakerBuyBaseVolume))
	assert.True(t, decimal.RequireFromString("50").Equal(mergedKCandle.TakerBuyQuoteVolume))
}

func TestKCandleBucketDomainDecidesOpenAndCloseByOpenTimeNotByPosition(t *testing.T) {
	bucketDomain := domains.NewKCandleBucketDomain(
		mustParseTime(t, "2026-09-02T10:00:00Z"),
		[]entities.KCandle{
			buildBucketKCandle(t, "2026-09-02T10:05:00Z", "120", "140", "90", "110", "7"),
			buildBucketKCandle(t, "2026-09-02T10:00:00Z", "100", "130", "95", "120", "3"),
		})

	mergedKCandle := bucketDomain.ToDto()

	assert.True(t, decimal.RequireFromString("100").Equal(mergedKCandle.Open))
	assert.True(t, decimal.RequireFromString("110").Equal(mergedKCandle.Close))
}

func TestKCandleBucketDomainHoldingOneCandleKeepsItsFiguresAsTheyAre(t *testing.T) {
	bucketStart := mustParseTime(t, "2026-09-02T10:00:00Z")
	bucketDomain := domains.NewKCandleBucketDomain(bucketStart, []entities.KCandle{
		buildBucketKCandle(t, "2026-09-02T10:35:00Z", "100", "110", "90", "105", "4"),
	})

	mergedKCandle := bucketDomain.ToDto()

	assert.Equal(t, bucketStart, mergedKCandle.OpenTime, "the bucket's start, not the candle's")
	assert.True(t, decimal.RequireFromString("100").Equal(mergedKCandle.Open))
	assert.True(t, decimal.RequireFromString("110").Equal(mergedKCandle.High))
	assert.True(t, decimal.RequireFromString("90").Equal(mergedKCandle.Low))
	assert.True(t, decimal.RequireFromString("105").Equal(mergedKCandle.Close))
	assert.True(t, decimal.RequireFromString("4").Equal(mergedKCandle.Volume))
}

func TestKCandleBucketDomainToVoIsTheSameMergeInTheShapeAScriptSees(t *testing.T) {
	bucketStart := mustParseTime(t, "2026-09-02T10:00:00Z")
	bucketDomain := domains.NewKCandleBucketDomain(bucketStart, []entities.KCandle{
		buildBucketKCandle(t, "2026-09-02T10:00:00Z", "100", "130", "95", "120", "3"),
		buildBucketKCandle(t, "2026-09-02T10:05:00Z", "120", "140", "90", "110", "7"),
	})

	kCandleVo := bucketDomain.ToVo()

	assert.Equal(t, "BTCUSDT", kCandleVo.Symbol)
	assert.InDelta(t, 100.0, kCandleVo.Open, 0.0001, "open comes from the earliest candle")
	assert.InDelta(t, 140.0, kCandleVo.High, 0.0001, "high is the highest high")
	assert.InDelta(t, 90.0, kCandleVo.Low, 0.0001, "low is the lowest low")
	assert.InDelta(t, 110.0, kCandleVo.Close, 0.0001, "close comes from the latest candle")
	assert.InDelta(t, 10.0, kCandleVo.Volume, 0.0001, "volume is the sum")
	assert.InDelta(t, 100.0, kCandleVo.QuoteVolume, 0.0001)
	assert.InDelta(t, 5.0, kCandleVo.TakerBuyBaseVolume, 0.0001)
	assert.InDelta(t, 50.0, kCandleVo.TakerBuyQuoteVolume, 0.0001)
}

func TestKCandleBucketDomainToVoCarriesTheBucketsOpenTimeAsSeconds(t *testing.T) {
	// A script is handed seconds rather than a time value, so that it cannot reach
	// the clock through one. The seconds are the bucket's own start, not a candle's.
	bucketStart := mustParseTime(t, "2026-09-02T10:00:00Z")
	bucketDomain := domains.NewKCandleBucketDomain(bucketStart, []entities.KCandle{
		buildBucketKCandle(t, "2026-09-02T10:35:00Z", "100", "110", "90", "105", "1"),
	})

	assert.Equal(t, bucketStart.Unix(), bucketDomain.ToVo().OpenTimeUnixSeconds)
}

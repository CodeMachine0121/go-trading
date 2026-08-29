package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func validWriteDto() dto.KCandleWriteDto {
	return dto.KCandleWriteDto{
		Symbol:              "BTCUSDT",
		OpenTime:            time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC),
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString("110"),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}
}

func TestNewKCandleDomainRejectsBrokenRules(t *testing.T) {
	currentTime := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	testCases := []struct {
		name           string
		mutate         func(writeDto *dto.KCandleWriteDto)
		expectedReason string
	}{
		{
			name:           "no trading symbol",
			mutate:         func(writeDto *dto.KCandleWriteDto) { writeDto.Symbol = "" },
			expectedReason: "必須指定交易標的",
		},
		{
			name: "open time off the five minute mark",
			mutate: func(writeDto *dto.KCandleWriteDto) {
				writeDto.OpenTime = time.Date(2026, 8, 29, 8, 3, 0, 0, time.UTC)
			},
			expectedReason: "起始時間必須落在5分鐘刻度上",
		},
		{
			name: "open time carrying seconds",
			mutate: func(writeDto *dto.KCandleWriteDto) {
				writeDto.OpenTime = time.Date(2026, 8, 29, 8, 5, 30, 0, time.UTC)
			},
			expectedReason: "起始時間必須落在5分鐘刻度上",
		},
		{
			name: "open time pointing into the future",
			mutate: func(writeDto *dto.KCandleWriteDto) {
				writeDto.OpenTime = time.Date(2026, 8, 29, 9, 5, 0, 0, time.UTC)
			},
			expectedReason: "起始時間不得指向未來",
		},
		{
			name: "highest price below the lowest price",
			mutate: func(writeDto *dto.KCandleWriteDto) {
				writeDto.High = decimal.RequireFromString("90")
				writeDto.Low = decimal.RequireFromString("100")
			},
			expectedReason: "最高價不得低於最低價",
		},
		{
			name:           "negative volume",
			mutate:         func(writeDto *dto.KCandleWriteDto) { writeDto.Volume = decimal.RequireFromString("-5") },
			expectedReason: "價格與成交數字不得為負數",
		},
		{
			name:           "negative open price",
			mutate:         func(writeDto *dto.KCandleWriteDto) { writeDto.Open = decimal.RequireFromString("-1") },
			expectedReason: "價格與成交數字不得為負數",
		},
		{
			name: "negative taker buy quote volume",
			mutate: func(writeDto *dto.KCandleWriteDto) {
				writeDto.TakerBuyQuoteVolume = decimal.RequireFromString("-1")
			},
			expectedReason: "價格與成交數字不得為負數",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validWriteDto()
			testCase.mutate(&writeDto)

			_, err := domains.NewKCandleDomain(writeDto, currentTime)

			assert.ErrorIs(t, err, domains.ErrKCandleValidation)
			assert.Contains(t, err.Error(), testCase.expectedReason)
		})
	}
}

func TestNewKCandleDomainAcceptsValidCandles(t *testing.T) {
	currentTime := time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC)

	testCases := []struct {
		name     string
		openTime time.Time
	}{
		{
			name:     "open time already in the past",
			openTime: time.Date(2026, 8, 29, 8, 55, 0, 0, time.UTC),
		},
		{
			name:     "open time exactly at the current time",
			openTime: time.Date(2026, 8, 29, 9, 0, 0, 0, time.UTC),
		},
		{
			name:     "open time on a mark other than the hour",
			openTime: time.Date(2026, 8, 29, 8, 45, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validWriteDto()
			writeDto.OpenTime = testCase.openTime

			kCandleDomain, err := domains.NewKCandleDomain(writeDto, currentTime)

			assert.NoError(t, err)
			kCandle := kCandleDomain.ToEntity()
			assert.Equal(t, "BTCUSDT", kCandle.Symbol)
			assert.Equal(t, testCase.openTime, kCandle.OpenTime)
			assert.True(t, decimal.RequireFromString("110").Equal(kCandle.Close))
			assert.True(t, decimal.RequireFromString("600").Equal(kCandle.TakerBuyQuoteVolume))
		})
	}
}

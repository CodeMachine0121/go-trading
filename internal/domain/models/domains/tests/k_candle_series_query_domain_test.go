package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const seriesQueryMaxBucketCount = 1000

func TestNewKCandleSeriesQueryDomainAcceptsARangeThatFitsTheLimit(t *testing.T) {
	testCases := []struct {
		name                      string
		startTime                 string
		endTime                   string
		interval                  string
		expectedSourceCandleLimit int
	}{
		{
			name:      "a range cut into exactly as many buckets as one query may answer with",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-05T11:15:00Z", interval: "5m",
			expectedSourceCandleLimit: 1000,
		},
		{
			name:      "the same start and end is one bucket",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T10:00:00Z", interval: "1h",
			expectedSourceCandleLimit: 12,
		},
		{
			name:      "a range too wide for five minutes fits comfortably at a day",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-05T11:20:00Z", interval: "1d",
			expectedSourceCandleLimit: 4 * 288,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:    "BTCUSDT",
				StartTime: mustParseTime(t, testCase.startTime),
				EndTime:   mustParseTime(t, testCase.endTime),
				Interval:  testCase.interval,
			}, seriesQueryMaxBucketCount)

			require.NoError(t, validationError)
			assert.Equal(t, "BTCUSDT", seriesQueryDomain.RangeQuery().Symbol())
			assert.Equal(t, mustParseTime(t, testCase.startTime), seriesQueryDomain.RangeQuery().StartTime())
			assert.Equal(t, mustParseTime(t, testCase.endTime), seriesQueryDomain.RangeQuery().EndTime())
			assert.Equal(t, testCase.expectedSourceCandleLimit, seriesQueryDomain.SourceCandleLimit())
		})
	}
}

func TestNewKCandleSeriesQueryDomainRefusesARangeCutIntoTooManyBuckets(t *testing.T) {
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T00:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-05T11:20:00Z"),
		Interval:  "5m",
	}, seriesQueryMaxBucketCount)

	require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "時間區間過大，請縮小區間或改用更長的彙總刻度（單次最多 1000 根）")
}

func TestNewKCandleSeriesQueryDomainRefusesWhatTheRangeQueryAlreadyRefuses(t *testing.T) {
	testCases := []struct {
		name                    string
		symbol                  string
		startTime               string
		endTime                 string
		expectedMessageFragment string
	}{
		{
			name: "no trading symbol", symbol: "",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T11:00:00Z",
			expectedMessageFragment: "必須指定交易標的",
		},
		{
			name: "ending before it starts", symbol: "BTCUSDT",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T09:00:00Z",
			expectedMessageFragment: "結束時間不得早於開始時間",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:    testCase.symbol,
				StartTime: mustParseTime(t, testCase.startTime),
				EndTime:   mustParseTime(t, testCase.endTime),
				Interval:  "1h",
			}, seriesQueryMaxBucketCount)

			require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessageFragment)
		})
	}
}

func TestNewKCandleSeriesQueryDomainRefusesAnIntervalNobodyOffers(t *testing.T) {
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T10:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-02T11:00:00Z"),
		Interval:  "7m",
	}, seriesQueryMaxBucketCount)

	require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "彙總刻度只能是")
}

func TestNewKCandleSeriesQueryDomainDeclaringNoIntervalMeansFiveMinutes(t *testing.T) {
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T10:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-02T10:55:00Z"),
	}, seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	assert.Equal(t, "5m", string(seriesQueryDomain.Interval().Value()))
	assert.Equal(t, 12, seriesQueryDomain.SourceCandleLimit())
}

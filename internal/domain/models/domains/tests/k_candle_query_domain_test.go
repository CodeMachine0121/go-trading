package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
)

func TestNewKCandleQueryDomain(t *testing.T) {
	startTime := time.Date(2026, 8, 29, 9, 1, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 29, 9, 9, 0, 0, time.UTC)

	testCases := []struct {
		name           string
		queryDto       dto.KCandleQueryDto
		expectedReason string
	}{
		{
			name:           "no trading symbol",
			queryDto:       dto.KCandleQueryDto{Symbol: "", StartTime: startTime, EndTime: endTime},
			expectedReason: "必須指定交易標的",
		},
		{
			name:           "end time earlier than start time",
			queryDto:       dto.KCandleQueryDto{Symbol: "BTCUSDT", StartTime: endTime, EndTime: startTime},
			expectedReason: "結束時間不得早於開始時間",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewKCandleQueryDomain(testCase.queryDto)

			assert.ErrorIs(t, err, domains.ErrKCandleValidation)
			assert.Contains(t, err.Error(), testCase.expectedReason)
		})
	}
}

func TestNewKCandleQueryDomainAcceptsValidQueries(t *testing.T) {
	testCases := []struct {
		name      string
		startTime time.Time
		endTime   time.Time
	}{
		{
			name:      "start and end off the five minute mark",
			startTime: time.Date(2026, 8, 29, 9, 1, 0, 0, time.UTC),
			endTime:   time.Date(2026, 8, 29, 9, 9, 0, 0, time.UTC),
		},
		{
			name:      "start and end at the same moment",
			startTime: time.Date(2026, 8, 29, 9, 5, 0, 0, time.UTC),
			endTime:   time.Date(2026, 8, 29, 9, 5, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			queryDomain, err := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
				Symbol: "BTCUSDT", StartTime: testCase.startTime, EndTime: testCase.endTime,
			})

			assert.NoError(t, err)
			assert.Equal(t, "BTCUSDT", queryDomain.Symbol())
			assert.Equal(t, testCase.startTime, queryDomain.StartTime())
			assert.Equal(t, testCase.endTime, queryDomain.EndTime())
		})
	}
}

// Reads normalise the symbol through the same model as writes, so a bot stored as BTCUSDT is found however it was typed.
func TestNewKCandleQueryDomainAsksInOneCase(t *testing.T) {
	startTime := time.Date(2026, 8, 29, 9, 1, 0, 0, time.UTC)
	endTime := time.Date(2026, 8, 29, 9, 9, 0, 0, time.UTC)

	testCases := []struct {
		name           string
		askedSymbol    string
		expectedSymbol string
	}{
		{name: "已經是大寫的原樣去問", askedSymbol: "BTCUSDT", expectedSymbol: "BTCUSDT"},
		{name: "小寫的改成大寫去問", askedSymbol: "btcusdt", expectedSymbol: "BTCUSDT"},
		{name: "大小寫混著的也改成大寫去問", askedSymbol: "BtcUsdt", expectedSymbol: "BTCUSDT"},
		{name: "沒有大小寫之分的台股代號原樣去問", askedSymbol: "0050", expectedSymbol: "0050"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			queryDomain, buildError := domains.NewKCandleQueryDomain(dto.KCandleQueryDto{
				Symbol: testCase.askedSymbol, StartTime: startTime, EndTime: endTime,
			})

			assert.NoError(t, buildError)
			assert.Equal(t, testCase.expectedSymbol, queryDomain.Symbol())
		})
	}
}

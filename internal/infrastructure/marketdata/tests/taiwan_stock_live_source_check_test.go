//go:build livecheck

// This file reaches the real venue to check what hand-written fixtures cannot, so it runs only on demand during a trading session:
//
//	TAIWAN_STOCK_API_KEY=... go test ./internal/infrastructure/marketdata/tests/ \
//	  -tags livecheck -run TestAgainstTheRealVenue -v -timeout 10m
package marketdata_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

const liveCheckDuration = 130 * time.Second

var liveCheckSymbols = []string{"0050", "2303", "2454", "00631L"}

var liveCheckTaipei = time.FixedZone("Asia/Taipei", 8*60*60)

// TestAgainstTheRealVenue checks that every minute the follow called finished matches the venue's own history exactly, since both come from the same source.
func TestAgainstTheRealVenue(t *testing.T) {
	apiKey := os.Getenv("TAIWAN_STOCK_API_KEY")
	require.NotEmpty(t, apiKey, "the venue needs its key")

	now := time.Now().In(liveCheckTaipei)
	sessionStart := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, liveCheckTaipei)
	sessionEnd := time.Date(now.Year(), now.Month(), now.Day(), 13, 30, 0, 0, liveCheckTaipei)
	require.True(t, now.After(sessionStart) && now.Before(sessionEnd),
		"this check only says anything while the market is trading; it is %s in Taipei",
		now.Format("15:04"))

	executionContext, stopFollowing := context.WithTimeout(
		context.Background(), liveCheckDuration)
	defer stopFollowing()

	liveKCandles, followError := marketdata.NewFugleIntradayLiveMarketDataProxy(
		"https://api.fugle.tw/marketdata/v1.0/stock/intraday/candles",
		apiKey, 5*time.Second, 55, 30*time.Second, 10*time.Second,
		marketdata.NewRequestPacer(55),
	).FollowKCandles(executionContext,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, liveCheckSymbols))
	require.NoError(t, followError)

	finished := map[string]map[time.Time]vo.LiveKCandleVo{}
	formingCount := 0
	for liveKCandle := range liveKCandles {
		if !liveKCandle.Closed {
			formingCount++

			continue
		}
		if finished[liveKCandle.Symbol] == nil {
			finished[liveKCandle.Symbol] = map[time.Time]vo.LiveKCandleVo{}
		}
		finished[liveKCandle.Symbol][liveKCandle.OpenTime.UTC()] = liveKCandle
	}

	t.Logf("followed %v for %s: %d updates in progress, %d minutes finished",
		liveCheckSymbols, liveCheckDuration, formingCount, len(finished))
	require.Positive(t, formingCount,
		"the feed delivered nothing at all — a caller would give it up for dead")

	compared := 0
	for _, symbol := range liveCheckSymbols {
		venueMinutes := venueCandles(t, apiKey, symbol)

		for openTime, liveKCandle := range finished[symbol] {
			venueKCandle, hasMinute := venueMinutes[openTime]
			require.True(t, hasMinute,
				"%s %s: reported finished, but the venue has no such minute",
				symbol, openTime)

			t.Logf("  %-7s %s  follow vol=%-8s close=%-8s | venue vol=%-8s close=%s",
				symbol, openTime.In(liveCheckTaipei).Format("15:04"),
				liveKCandle.Volume, liveKCandle.Close,
				venueKCandle.volume, venueKCandle.close)

			require.True(t, liveKCandle.Volume.Equal(venueKCandle.volume),
				"%s %s: volume differs — follow %s, venue %s",
				symbol, openTime, liveKCandle.Volume, venueKCandle.volume)
			require.True(t, liveKCandle.Close.Equal(venueKCandle.close),
				"%s %s: close differs — follow %s, venue %s",
				symbol, openTime, liveKCandle.Close, venueKCandle.close)

			compared++
		}
	}

	t.Logf("compared %d finished minutes against the venue, all identical", compared)
	require.Positive(t, compared, "nothing finished; run this for longer")
}

type venueKCandle struct {
	volume decimal.Decimal
	close  decimal.Decimal
}

func venueCandles(t *testing.T, apiKey string, symbol string) map[time.Time]venueKCandle {
	t.Helper()

	request, buildError := http.NewRequest(http.MethodGet, fmt.Sprintf(
		"https://api.fugle.tw/marketdata/v1.0/stock/intraday/candles/%s?timeframe=1",
		symbol), nil)
	require.NoError(t, buildError)
	request.Header.Set("X-API-KEY", apiKey)

	response, requestError := http.DefaultClient.Do(request)
	require.NoError(t, requestError)
	defer func() { _ = response.Body.Close() }()

	var answer struct {
		Data []struct {
			Date   string          `json:"date"`
			Close  decimal.Decimal `json:"close"`
			Volume decimal.Decimal `json:"volume"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(response.Body).Decode(&answer))

	minutes := map[time.Time]venueKCandle{}
	for _, reported := range answer.Data {
		reportedTime, parseError := time.Parse(time.RFC3339, reported.Date)
		require.NoError(t, parseError)
		minutes[reportedTime.UTC()] = venueKCandle{
			volume: reported.Volume, close: reported.Close,
		}
	}

	return minutes
}

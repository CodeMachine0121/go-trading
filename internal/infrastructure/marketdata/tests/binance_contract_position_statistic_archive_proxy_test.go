package marketdata_test

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const archiveHeader = "create_time,symbol,sum_open_interest,sum_open_interest_value," +
	"count_toptrader_long_short_ratio,sum_toptrader_long_short_ratio,count_long_short_ratio," +
	"sum_taker_long_short_vol_ratio\n"

var archivedDayAsked = time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

func zippedArchiveDay(t *testing.T, csvContent string) []byte {
	t.Helper()

	var zipped bytes.Buffer
	zipWriter := zip.NewWriter(&zipped)
	csvFile, createError := zipWriter.Create("ETHUSDT-metrics-2026-03-01.csv")
	require.NoError(t, createError)
	_, writeError := csvFile.Write([]byte(csvContent))
	require.NoError(t, writeError)
	require.NoError(t, zipWriter.Close())

	return zipped.Bytes()
}

// withUnknownCompression marks the zip's one file as compressed by a method nobody
// knows, in both places a zip names it: the file's own header and the directory.
func withUnknownCompression(zipped []byte) []byte {
	marked := bytes.Clone(zipped)
	for _, header := range []struct {
		signature    []byte
		methodOffset int
	}{
		{signature: []byte{0x50, 0x4b, 0x03, 0x04}, methodOffset: 8},
		{signature: []byte{0x50, 0x4b, 0x01, 0x02}, methodOffset: 10},
	} {
		at := bytes.Index(marked, header.signature)
		marked[at+header.methodOffset] = 99
		marked[at+header.methodOffset+1] = 0
	}

	return marked
}

// archiveHost answers every file with one status and body, and keeps the paths it was
// asked for.
type archiveHost struct {
	baseUrl string
	lock    *sync.Mutex
	paths   *[]string
	headers *[]http.Header
}

func servedByArchive(t *testing.T, status int, body []byte) archiveHost {
	t.Helper()

	host := archiveHost{lock: &sync.Mutex{}, paths: &[]string{}, headers: &[]http.Header{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		host.lock.Lock()
		*host.paths = append(*host.paths, request.URL.Path)
		*host.headers = append(*host.headers, request.Header.Clone())
		host.lock.Unlock()

		writer.WriteHeader(status)
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)
	host.baseUrl = server.URL + "/data/futures/um/daily/metrics"

	return host
}

func (host archiveHost) proxy() *marketdata.BinanceContractPositionStatisticArchiveProxy {
	return marketdata.NewBinanceContractPositionStatisticArchiveProxy(
		host.baseUrl, 5*time.Second, marketdata.NewRequestPacer(0))
}

func TestArchiveProxyReadsEveryStatisticOfTheDayOldestFirst(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader+
		"2026-03-01 00:00:00,ETHUSDT,1794474.325,3528901866.675849,2.90719776,1.362087,2.26952564,1.016569\n"+
		"2026-03-01 00:05:00,ETHUSDT,1793061.383,3513539641.21616,2.8981404,1.362704,2.25912179,0.69297\n"))

	statistics, found, fetchError := host.proxy().FetchDailyPositionStatistics(
		t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	assert.True(t, found)
	require.Len(t, statistics, 2)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), statistics[0].StatisticTime)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 5, 0, 0, time.UTC), statistics[1].StatisticTime)
	assert.Equal(t, "ETHUSDT", statistics[1].Symbol)
	assert.True(t, decimal.RequireFromString("1793061.383").Equal(statistics[1].OpenInterest.Decimal))
	assert.True(t, decimal.RequireFromString("3513539641.21616").Equal(statistics[1].OpenInterestValue.Decimal))
	// The account ratio is the one counted across every account; the largest
	// accounts' ratio is the one counted by position, not by account.
	assert.True(t, decimal.RequireFromString("2.25912179").Equal(statistics[1].AccountLongShortRatio.Decimal))
	assert.True(t, decimal.RequireFromString("1.362704").Equal(statistics[1].TopTraderPositionLongShortRatio.Decimal))
}

func TestArchiveProxyAsksForTheContractsFileOfThatDay(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader))

	_, _, fetchError := host.proxy().FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	assert.Equal(t, []string{"/data/futures/um/daily/metrics/ETHUSDT/ETHUSDT-metrics-2026-03-01.zip"}, *host.paths)
}

func TestArchiveProxyAsksWithoutAnyAccount(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader))

	_, _, fetchError := host.proxy().FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	require.Len(t, *host.headers, 1)
	asked := (*host.headers)[0]
	for _, credentialHeader := range []string{"Authorization", "X-Mbx-Apikey", "Cookie"} {
		assert.Empty(t, asked.Get(credentialHeader), "歷史資料庫是公開的，不帶 %s", credentialHeader)
	}
}

func TestArchiveProxyAnswersADayWithNoFileAsNotFound(t *testing.T) {
	host := servedByArchive(t, http.StatusNotFound, []byte("<Error><Code>NoSuchKey</Code></Error>"))

	statistics, found, fetchError := host.proxy().FetchDailyPositionStatistics(
		t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	assert.False(t, found)
	assert.Empty(t, statistics)
}

func TestArchiveProxyLeavesABlankFigureAbsent(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader+
		"2026-03-01 00:00:00,ETHUSDT,1794474.325,3528901866.675849,2.90719776,,2.26952564,1.016569\n"))

	statistics, found, fetchError := host.proxy().FetchDailyPositionStatistics(
		t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	assert.True(t, found)
	require.Len(t, statistics, 1)
	assert.False(t, statistics[0].TopTraderPositionLongShortRatio.Valid)
	assert.True(t, statistics[0].AccountLongShortRatio.Valid)
	assert.True(t, statistics[0].OpenInterest.Valid)
	assert.True(t, statistics[0].OpenInterestValue.Valid)
}

func TestArchiveProxyFindsColumnsByTheirHeader(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t,
		"sum_toptrader_long_short_ratio,count_long_short_ratio,sum_open_interest_value,sum_open_interest,create_time\n"+
			"1.5,2.5,200,100,2026-03-01 00:10:00\n"))

	statistics, _, fetchError := host.proxy().FetchDailyPositionStatistics(
		t.Context(), "ETHUSDT", archivedDayAsked)

	require.NoError(t, fetchError)
	require.Len(t, statistics, 1)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 10, 0, 0, time.UTC), statistics[0].StatisticTime)
	assert.True(t, decimal.RequireFromString("100").Equal(statistics[0].OpenInterest.Decimal))
	assert.True(t, decimal.RequireFromString("200").Equal(statistics[0].OpenInterestValue.Decimal))
	assert.True(t, decimal.RequireFromString("2.5").Equal(statistics[0].AccountLongShortRatio.Decimal))
	assert.True(t, decimal.RequireFromString("1.5").Equal(statistics[0].TopTraderPositionLongShortRatio.Decimal))
}

func TestArchiveProxyRefusesWhatItCannotRead(t *testing.T) {
	testCases := []struct {
		name   string
		status int
		body   func(t *testing.T) []byte
	}{
		// A readable day behind a status other than success is still not an answer.
		{name: "來源回錯誤", status: http.StatusInternalServerError, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, archiveHeader)
		}},
		{name: "來源拒絕", status: http.StatusForbidden, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, archiveHeader)
		}},
		{name: "不是 zip", status: http.StatusOK,
			body: func(*testing.T) []byte { return []byte("not a zip") }},
		{name: "空的 zip", status: http.StatusOK, body: func(t *testing.T) []byte {
			var zipped bytes.Buffer
			require.NoError(t, zip.NewWriter(&zipped).Close())

			return zipped.Bytes()
		}},
		{name: "沒有表頭", status: http.StatusOK,
			body: func(t *testing.T) []byte { return zippedArchiveDay(t, "") }},
		// Every other cell of this file would read as a number, so only noticing the
		// missing column refuses it.
		{name: "少了需要的欄位", status: http.StatusOK, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, "sum_open_interest,create_time,sum_open_interest_value,count_long_short_ratio\n"+
				"1,2026-03-01 00:00:00,2,3\n")
		}},
		{name: "壓縮方式讀不懂", status: http.StatusOK, body: func(t *testing.T) []byte {
			return withUnknownCompression(zippedArchiveDay(t, archiveHeader))
		}},
		{name: "數字讀不懂", status: http.StatusOK, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, archiveHeader+
				"2026-03-01 00:00:00,ETHUSDT,lots,3528901866.675849,2.9,1.36,2.26,1.01\n")
		}},
		{name: "時間讀不懂", status: http.StatusOK, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, archiveHeader+
				"yesterday,ETHUSDT,1794474.325,3528901866.675849,2.9,1.36,2.26,1.01\n")
		}},
		{name: "某一列欄位數不對", status: http.StatusOK, body: func(t *testing.T) []byte {
			return zippedArchiveDay(t, archiveHeader+"2026-03-01 00:00:00,ETHUSDT\n")
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			host := servedByArchive(t, testCase.status, testCase.body(t))

			statistics, found, fetchError := host.proxy().FetchDailyPositionStatistics(
				t.Context(), "ETHUSDT", archivedDayAsked)

			require.Error(t, fetchError)
			assert.False(t, found)
			assert.Empty(t, statistics)
		})
	}
}

func TestArchiveProxyRefusesWhenTheAnswerIsCutShort(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "1000")
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("PK"))
	}))
	t.Cleanup(server.Close)
	proxy := marketdata.NewBinanceContractPositionStatisticArchiveProxy(
		server.URL, 5*time.Second, marketdata.NewRequestPacer(0))

	_, found, fetchError := proxy.FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

	require.Error(t, fetchError)
	assert.False(t, found)
}

func TestArchiveProxyRefusesAnAddressItCannotAskAbout(t *testing.T) {
	proxy := marketdata.NewBinanceContractPositionStatisticArchiveProxy(
		"http://archive\x7f.invalid", time.Second, marketdata.NewRequestPacer(0))

	_, found, fetchError := proxy.FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

	require.Error(t, fetchError)
	assert.False(t, found)
}

func TestArchiveProxyStopsWhenTheCallerHasGivenUp(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader))
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	_, found, fetchError := host.proxy().FetchDailyPositionStatistics(cancelled, "ETHUSDT", archivedDayAsked)

	require.ErrorIs(t, fetchError, context.Canceled)
	assert.False(t, found)
	assert.Empty(t, *host.paths)
}

func TestArchiveProxyRefusesWhenTheArchiveCannotBeReached(t *testing.T) {
	unreachable := marketdata.NewBinanceContractPositionStatisticArchiveProxy(
		"http://127.0.0.1:1", time.Second, marketdata.NewRequestPacer(0))

	_, found, fetchError := unreachable.FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

	require.Error(t, fetchError)
	assert.False(t, found)
}

func TestArchiveProxyNamesTheSameCellEveryTimeARowIsWrongInTwo(t *testing.T) {
	host := servedByArchive(t, http.StatusOK, zippedArchiveDay(t, archiveHeader+
		"2026-03-01 00:00:00,ETHUSDT,lots,plenty,2.9,1.36,2.26,1.01\n"))

	for range 20 {
		_, _, fetchError := host.proxy().FetchDailyPositionStatistics(t.Context(), "ETHUSDT", archivedDayAsked)

		require.Error(t, fetchError)
		assert.Contains(t, fetchError.Error(), "read sum_open_interest at")
	}
}

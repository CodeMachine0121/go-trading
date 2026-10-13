package marketdata_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const tradeArchiveHeader = "agg_trade_id,price,quantity,first_trade_id,last_trade_id,transact_time,is_buyer_maker\n"

var tradeDay = time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC)

func deflatedTradeArchive(t *testing.T, csvContent string) []byte {
	t.Helper()

	zipped := bytes.Buffer{}
	zipWriter := zip.NewWriter(&zipped)
	csvFile, createError := zipWriter.Create("BTCUSDT-aggTrades-2026-10-13.csv")
	require.NoError(t, createError)
	_, writeError := csvFile.Write([]byte(csvContent))
	require.NoError(t, writeError)
	require.NoError(t, zipWriter.Close())

	return zipped.Bytes()
}

// storedTradeArchive keeps the CSV uncompressed with its size written up front, the other way a zip may hold it.
func storedTradeArchive(t *testing.T, csvContent string) []byte {
	t.Helper()

	zipped := bytes.Buffer{}
	zipWriter := zip.NewWriter(&zipped)
	csvFile, createError := zipWriter.CreateRaw(&zip.FileHeader{
		Name:               "BTCUSDT-aggTrades-2026-10-13.csv",
		Method:             zip.Store,
		CRC32:              crc32.ChecksumIEEE([]byte(csvContent)),
		CompressedSize64:   uint64(len(csvContent)),
		UncompressedSize64: uint64(len(csvContent)),
	})
	require.NoError(t, createError)
	_, writeError := csvFile.Write([]byte(csvContent))
	require.NoError(t, writeError)
	require.NoError(t, zipWriter.Close())

	return zipped.Bytes()
}

type tradeHost struct {
	url      string
	lock     *sync.Mutex
	requests *[]*http.Request
}

func (host tradeHost) asked() []*http.Request {
	host.lock.Lock()
	defer host.lock.Unlock()

	return append([]*http.Request(nil), *host.requests...)
}

// servedTrades answers each request with the next of the given handlers, recording what was asked.
func servedTrades(t *testing.T, answers ...func(http.ResponseWriter, *http.Request)) tradeHost {
	t.Helper()

	host := tradeHost{lock: &sync.Mutex{}, requests: &[]*http.Request{}}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		host.lock.Lock()
		*host.requests = append(*host.requests, request)
		answerIndex := len(*host.requests) - 1
		host.lock.Unlock()

		if answerIndex >= len(answers) {
			t.Errorf("unexpected request %s", request.URL)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		answers[answerIndex](writer, request)
	}))
	t.Cleanup(server.Close)
	host.url = server.URL

	return host
}

func answering(status int, body []byte) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
		_, _ = writer.Write(body)
	}
}

func tradeHistoryProxy(archiveUrl string, liveUrl string, concurrentDownloads int) *marketdata.BinanceContractTradeHistoryProxy {
	return marketdata.NewBinanceContractTradeHistoryProxy(
		archiveUrl, liveUrl, 5*time.Second, 5*time.Second,
		marketdata.NewRequestPacer(0), marketdata.NewRequestPacer(0), concurrentDownloads)
}

func collecting(trades *[]vo.ContractAggregatedTradeVo) func(vo.ContractAggregatedTradeVo) {
	return func(trade vo.ContractAggregatedTradeVo) { *trades = append(*trades, trade) }
}

func TestADailyArchiveIsReadTradeByTrade(t *testing.T) {
	host := servedTrades(t, answering(http.StatusOK, deflatedTradeArchive(t, tradeArchiveHeader+
		"1,84157.10,0.002,10,11,1791850200000,false\n"+
		"2,84151.00,0.001,12,12,1791850200500,true\n")))
	trades := []vo.ContractAggregatedTradeVo{}

	found, visitError := tradeHistoryProxy(host.url+"/aggTrades", "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, collecting(&trades))

	require.NoError(t, visitError)
	assert.True(t, found)
	assert.Equal(t, "/aggTrades/BTCUSDT/BTCUSDT-aggTrades-2026-10-13.zip", host.asked()[0].URL.Path)
	assert.Equal(t, []vo.ContractAggregatedTradeVo{
		{Price: decimal.RequireFromString("84157.10"), Quantity: 0.002, TradeCount: 2,
			TradeTime: time.UnixMilli(1791850200000).UTC(), BuyerIsTaker: true},
		{Price: decimal.RequireFromString("84151.00"), Quantity: 0.001, TradeCount: 1,
			TradeTime: time.UnixMilli(1791850200500).UTC(), BuyerIsTaker: false},
	}, trades)
}

func TestAnOlderArchiveWithoutAHeaderRowIsReadFromItsFirstRow(t *testing.T) {
	host := servedTrades(t, answering(http.StatusOK, storedTradeArchive(t,
		"1,100,1,10,10,1791850200000,false\n")))
	trades := []vo.ContractAggregatedTradeVo{}

	found, visitError := tradeHistoryProxy(host.url, "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, collecting(&trades))

	require.NoError(t, visitError)
	assert.True(t, found)
	require.Len(t, trades, 1)
	assert.True(t, decimal.NewFromInt(100).Equal(trades[0].Price))
}

func TestAnArchiveStampedInMicrosecondsReadsTheSameMoment(t *testing.T) {
	host := servedTrades(t, answering(http.StatusOK, deflatedTradeArchive(t,
		"1,100,1,10,10,1791850200000123,false\n")))
	trades := []vo.ContractAggregatedTradeVo{}

	_, visitError := tradeHistoryProxy(host.url, "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, collecting(&trades))

	require.NoError(t, visitError)
	assert.Equal(t, time.UnixMicro(1791850200000123).UTC(), trades[0].TradeTime)
}

func TestADayWithoutAnArchiveYetIsNotFound(t *testing.T) {
	host := servedTrades(t, answering(http.StatusNotFound, nil))
	trades := []vo.ContractAggregatedTradeVo{}

	found, visitError := tradeHistoryProxy(host.url, "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, collecting(&trades))

	require.NoError(t, visitError)
	assert.False(t, found)
	assert.Empty(t, trades)
}

func TestAnArchiveThatCannotBeReadIsAnError(t *testing.T) {
	deflated := deflatedTradeArchive(t, "1,100,1,10,10,1791850200000,false\n")
	unknownCompression := bytes.Clone(deflated)
	unknownCompression[8] = 99

	testCases := []struct {
		name             string
		answer           func(http.ResponseWriter, *http.Request)
		expectedFragment string
	}{
		{name: "來源回錯誤", answer: answering(http.StatusInternalServerError, nil), expectedFragment: "archive answered 500"},
		{name: "不是 zip", answer: answering(http.StatusOK, []byte("<html>nope</html>..........................")), expectedFragment: "not a zip file"},
		{name: "太短", answer: answering(http.StatusOK, []byte("PK")), expectedFragment: "read zip header"},
		{name: "看不懂的壓縮", answer: answering(http.StatusOK, unknownCompression), expectedFragment: "unsupported zip compression 99"},
		{name: "欄位不夠", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,100,1\n")), expectedFragment: "row 1 has 3 columns"},
		{name: "價格讀不懂", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,abc,1,10,10,1791850200000,false\n")), expectedFragment: `price "abc"`},
		{name: "數量讀不懂", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,100,x,10,10,1791850200000,false\n")), expectedFragment: `quantity "x"`},
		{name: "成交編號讀不懂", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,100,1,a,10,1791850200000,false\n")), expectedFragment: "trade identifiers"},
		{name: "時間讀不懂", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,100,1,10,10,soon,false\n")), expectedFragment: `trade time "soon"`},
		{name: "引號壞掉", answer: answering(http.StatusOK, deflatedTradeArchive(t, "1,\"100,1,10,10,1,false\n")), expectedFragment: "row 1"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			host := servedTrades(t, testCase.answer)

			found, visitError := tradeHistoryProxy(host.url, "", 1).
				VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})

			require.Error(t, visitError)
			assert.False(t, found)
			assert.Contains(t, visitError.Error(), testCase.expectedFragment)
		})
	}
}

func TestAnUnreachableArchiveIsAnError(t *testing.T) {
	_, visitError := tradeHistoryProxy("http://127.0.0.1:1", "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})

	require.Error(t, visitError)
	assert.Contains(t, visitError.Error(), "read trade archive for BTCUSDT on 2026-10-13")
}

func TestArchiveDownloadsWaitForAFreeSlot(t *testing.T) {
	release := make(chan struct{})
	host := servedTrades(t, func(writer http.ResponseWriter, _ *http.Request) {
		<-release
		writer.WriteHeader(http.StatusNotFound)
	})
	proxy := tradeHistoryProxy(host.url, "", 1)
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = proxy.VisitDailyTrades(context.Background(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})
	}()
	require.Eventually(t, func() bool { return len(host.asked()) == 1 }, time.Second, 5*time.Millisecond)

	waitingContext, stopWaiting := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stopWaiting()
	_, secondError := proxy.VisitDailyTrades(waitingContext, "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})
	close(release)
	<-firstDone

	require.ErrorIs(t, secondError, context.DeadlineExceeded)
	assert.Len(t, host.asked(), 1, "第二份在第一份下載完之前不會開始")
}

type liveTrade struct {
	ID int64  `json:"a"`
	P  string `json:"p"`
	Q  string `json:"q"`
	F  int64  `json:"f"`
	L  int64  `json:"l"`
	T  int64  `json:"T"`
	M  bool   `json:"m"`
}

func livePage(t *testing.T, trades ...liveTrade) func(http.ResponseWriter, *http.Request) {
	t.Helper()

	body, encodeError := json.Marshal(trades)
	require.NoError(t, encodeError)

	return answering(http.StatusOK, body)
}

// fullLivePage is a page as long as the venue ever answers, trades one millisecond apart from the given identifier and moment.
func fullLivePage(t *testing.T, firstID int64, firstMoment time.Time) func(http.ResponseWriter, *http.Request) {
	t.Helper()

	trades := make([]liveTrade, 0, 1000)
	for offset := range int64(1000) {
		trades = append(trades, liveTrade{ID: firstID + offset, P: "100", Q: "1", F: 1, L: 1,
			T: firstMoment.Add(time.Duration(offset) * time.Millisecond).UnixMilli()})
	}

	return livePage(t, trades...)
}

var liveStart = time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)

func TestLiveTradesAreAskedForAnHourAtATime(t *testing.T) {
	host := servedTrades(t,
		livePage(t, liveTrade{ID: 1, P: "84157", Q: "2", F: 5, L: 6, T: liveStart.Add(time.Minute).UnixMilli(), M: false}),
		livePage(t))
	trades := []vo.ContractAggregatedTradeVo{}

	visitError := tradeHistoryProxy("", host.url+"/aggTrades", 1).
		VisitTradesBetween(t.Context(), "BTCUSDT", liveStart, liveStart.Add(90*time.Minute), collecting(&trades))

	require.NoError(t, visitError)
	asked := host.asked()
	require.Len(t, asked, 2)
	assert.Equal(t, "/aggTrades", asked[0].URL.Path)
	assert.Equal(t, "BTCUSDT", asked[0].URL.Query().Get("symbol"))
	assert.Equal(t, "1000", asked[0].URL.Query().Get("limit"))
	assert.Equal(t, fmt.Sprint(liveStart.UnixMilli()), asked[0].URL.Query().Get("startTime"))
	assert.Equal(t, fmt.Sprint(liveStart.Add(time.Hour).UnixMilli()-1), asked[0].URL.Query().Get("endTime"))
	assert.Equal(t, fmt.Sprint(liveStart.Add(time.Hour).UnixMilli()), asked[1].URL.Query().Get("startTime"))
	assert.Equal(t, fmt.Sprint(liveStart.Add(90*time.Minute).UnixMilli()-1), asked[1].URL.Query().Get("endTime"))
	assert.Equal(t, []vo.ContractAggregatedTradeVo{{Price: decimal.RequireFromString("84157"), Quantity: 2,
		TradeCount: 2, TradeTime: liveStart.Add(time.Minute), BuyerIsTaker: true}}, trades)
}

func TestAFullLivePageIsReadOnByIdentifierUntilTheEnd(t *testing.T) {
	end := liveStart.Add(30 * time.Minute)
	host := servedTrades(t,
		fullLivePage(t, 1, liveStart),
		livePage(t,
			liveTrade{ID: 1001, P: "100", Q: "1", F: 1, L: 1, T: end.Add(-time.Millisecond).UnixMilli()},
			liveTrade{ID: 1002, P: "100", Q: "1", F: 1, L: 1, T: end.UnixMilli()}))
	trades := []vo.ContractAggregatedTradeVo{}

	visitError := tradeHistoryProxy("", host.url, 1).
		VisitTradesBetween(t.Context(), "BTCUSDT", liveStart, end, collecting(&trades))

	require.NoError(t, visitError)
	asked := host.asked()
	require.Len(t, asked, 2)
	assert.Equal(t, "1001", asked[1].URL.Query().Get("fromId"))
	assert.Empty(t, asked[1].URL.Query().Get("startTime"))
	assert.Len(t, trades, 1001, "收盤那一刻的那一筆不算")
}

func TestPagingByIdentifierStopsAtTheNewestTrade(t *testing.T) {
	host := servedTrades(t, fullLivePage(t, 1, liveStart), livePage(t))
	trades := []vo.ContractAggregatedTradeVo{}

	visitError := tradeHistoryProxy("", host.url, 1).
		VisitTradesBetween(t.Context(), "BTCUSDT", liveStart, liveStart.Add(3*time.Hour), collecting(&trades))

	require.NoError(t, visitError)
	assert.Len(t, host.asked(), 2, "沒有更新的成交了，不再問後面的時窗")
	assert.Len(t, trades, 1000)
}

func TestLiveTradesThatCannotBeReadAreAnError(t *testing.T) {
	testCases := []struct {
		name             string
		answers          []func(http.ResponseWriter, *http.Request)
		expectedFragment string
	}{
		{name: "來源回錯誤", answers: []func(http.ResponseWriter, *http.Request){answering(http.StatusTooManyRequests, nil)}, expectedFragment: "live trades answered 429 for BTCUSDT"},
		{name: "讀不懂", answers: []func(http.ResponseWriter, *http.Request){answering(http.StatusOK, []byte("{"))}, expectedFragment: "read live trades for BTCUSDT"},
		{name: "價格讀不懂", answers: []func(http.ResponseWriter, *http.Request){livePage(t, liveTrade{ID: 1, P: "x", Q: "1", T: liveStart.UnixMilli()})}, expectedFragment: `price "x"`},
		{name: "續頁時出錯", answers: []func(http.ResponseWriter, *http.Request){fullLivePage(t, 1, liveStart), answering(http.StatusBadGateway, nil)}, expectedFragment: "live trades answered 502"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			host := servedTrades(t, testCase.answers...)

			visitError := tradeHistoryProxy("", host.url, 1).VisitTradesBetween(
				t.Context(), "BTCUSDT", liveStart, liveStart.Add(time.Hour), func(vo.ContractAggregatedTradeVo) {})

			require.Error(t, visitError)
			assert.Contains(t, visitError.Error(), testCase.expectedFragment)
		})
	}
}

func TestUnreachableLiveTradesAreAnError(t *testing.T) {
	visitError := tradeHistoryProxy("", "http://127.0.0.1:1", 1).VisitTradesBetween(
		t.Context(), "BTCUSDT", liveStart, liveStart.Add(time.Hour), func(vo.ContractAggregatedTradeVo) {})

	require.Error(t, visitError)
	assert.Contains(t, visitError.Error(), "reach live trades for BTCUSDT")
}

func TestAStoppedRunAsksNothing(t *testing.T) {
	host := servedTrades(t)
	stopped, stop := context.WithCancel(t.Context())
	stop()

	liveError := tradeHistoryProxy(host.url, host.url, 1).VisitTradesBetween(
		stopped, "BTCUSDT", liveStart, liveStart.Add(time.Hour), func(vo.ContractAggregatedTradeVo) {})

	require.ErrorIs(t, liveError, context.Canceled)
	assert.Empty(t, host.asked())
}

func TestAZipCutOffAfterItsHeaderIsAnError(t *testing.T) {
	host := servedTrades(t, answering(http.StatusOK, deflatedTradeArchive(t, "1,100,1,10,10,1,false\n")[:32]))

	_, visitError := tradeHistoryProxy(host.url, "", 1).
		VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})

	require.Error(t, visitError)
	assert.Contains(t, visitError.Error(), "read zip header")
}

func TestAMalformedAddressIsAnError(t *testing.T) {
	proxy := tradeHistoryProxy(":%", ":%", 1)

	_, archiveError := proxy.VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})
	liveError := proxy.VisitTradesBetween(t.Context(), "BTCUSDT", liveStart, liveStart.Add(time.Hour), func(vo.ContractAggregatedTradeVo) {})

	assert.ErrorContains(t, archiveError, "read trade archive for BTCUSDT")
	assert.ErrorContains(t, liveError, "reach live trades for BTCUSDT")
}

func TestARequestThatWouldWaitPastItsDeadlineForItsTurnIsNotSent(t *testing.T) {
	host := servedTrades(t, answering(http.StatusNotFound, nil), livePage(t))
	// One request a minute: the first goes at once, the second would wait far past a short deadline.
	proxy := marketdata.NewBinanceContractTradeHistoryProxy(host.url, host.url, time.Second, time.Second,
		marketdata.NewRequestPacer(1), marketdata.NewRequestPacer(1), 1)
	_, _ = proxy.VisitDailyTrades(t.Context(), "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})
	_ = proxy.VisitTradesBetween(t.Context(), "BTCUSDT", liveStart, liveStart.Add(time.Minute), func(vo.ContractAggregatedTradeVo) {})

	shortDeadline, stop := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer stop()
	_, archiveError := proxy.VisitDailyTrades(shortDeadline, "BTCUSDT", tradeDay, func(vo.ContractAggregatedTradeVo) {})
	liveError := proxy.VisitTradesBetween(shortDeadline, "BTCUSDT", liveStart, liveStart.Add(time.Minute), func(vo.ContractAggregatedTradeVo) {})

	assert.Error(t, archiveError)
	assert.Error(t, liveError)
	assert.Len(t, host.asked(), 2, "等不到輪次的那兩次沒有送出")
}

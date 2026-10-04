package exchange_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/exchange"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// receivedRequest is what the fake venue saw.
type receivedRequest struct {
	method     string
	path       string
	rawQuery   string
	query      url.Values
	apiKeyHead string
}

// contractVenueAnswering is a fake venue that answers every request the same way and remembers each one.
func contractVenueAnswering(
	t *testing.T, statusCode int, body string,
) (*exchange.BinanceContractOrderProxy, *[]receivedRequest) {
	received := []receivedRequest{}
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			received = append(received, receivedRequest{
				method: request.Method, path: request.URL.Path, rawQuery: request.URL.RawQuery,
				query: request.URL.Query(), apiKeyHead: request.Header.Get("X-MBX-APIKEY"),
			})
			writer.WriteHeader(statusCode)
			_, _ = writer.Write([]byte(body))
		}))
	t.Cleanup(server.Close)

	return exchange.NewBinanceContractOrderProxy(server.URL, server.Client(), aClockAt(t, verificationMoment)), &received
}

var aMarketBuy = vo.ContractMarketOrderVo{
	Symbol: "BTCUSDT", Side: vo.ContractOrderSideBuy, Quantity: decimal.RequireFromString("0.002"),
	ClientOrderID: "gt-ao-41-open",
}

func TestBinanceContractOrderProxyPlacesASignedMarketOrderAndReadsTheFill(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK,
		`{"status":"FILLED","executedQty":"0.002","avgPrice":"85000.0"}`)

	fill, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.True(t, fill.ExecutedQuantity.Equal(decimal.RequireFromString("0.002")))
	assert.True(t, fill.AveragePrice.Equal(decimal.NewFromInt(85000)))
	require.Len(t, *received, 1)
	request := (*received)[0]
	assert.Equal(t, http.MethodPost, request.method)
	assert.Equal(t, "/fapi/v1/order", request.path)
	assert.Equal(t, "the-api-key", request.apiKeyHead)
	assert.Equal(t, "MARKET", request.query.Get("type"))
	assert.Equal(t, "BUY", request.query.Get("side"))
	assert.Equal(t, "0.002", request.query.Get("quantity"))
	assert.Equal(t, "gt-ao-41-open", request.query.Get("newClientOrderId"))
	assert.Empty(t, request.query.Get("reduceOnly"))
	unsignedQuery, signature, hasSignature := strings.Cut(request.rawQuery, "&signature=")
	require.True(t, hasSignature)
	signer := hmac.New(sha256.New, []byte("the-secret-key"))
	signer.Write([]byte(unsignedQuery))
	assert.Equal(t, hex.EncodeToString(signer.Sum(nil)), signature)
	assert.NotContains(t, request.rawQuery, "the-secret-key")
}

func TestBinanceContractOrderProxyMarksACloseAsReduceOnly(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK,
		`{"status":"FILLED","executedQty":"0.002","avgPrice":"85000"}`)
	closing := aMarketBuy
	closing.ReduceOnly = true

	proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, closing)

	assert.Equal(t, "true", (*received)[0].query.Get("reduceOnly"))
}

func TestBinanceContractOrderProxySortsTheVenuesRefusals(t *testing.T) {
	testCases := []struct {
		name            string
		statusCode      int
		body            string
		expectedFailure vo.ContractOrderFailureVo
	}{
		{name: "餘額不足", statusCode: http.StatusBadRequest, body: `{"code":-2019,"msg":"Margin is insufficient."}`,
			expectedFailure: vo.ContractOrderFailureInsufficientBalance},
		{name: "未授權", statusCode: http.StatusUnauthorized, body: `{}`,
			expectedFailure: vo.ContractOrderFailureKeyRejected},
		{name: "金鑰或權限不符", statusCode: http.StatusBadRequest, body: `{"code":-2015,"msg":"Invalid API-key, IP, or permissions for action."}`,
			expectedFailure: vo.ContractOrderFailureKeyRejected},
		{name: "最小名目", statusCode: http.StatusBadRequest, body: `{"code":-4164,"msg":"Order's notional must be no smaller than 5"}`,
			expectedFailure: vo.ContractOrderFailureVenueRefused},
		{name: "精度", statusCode: http.StatusBadRequest, body: `{"code":-1111,"msg":"Precision is over the maximum defined for this asset."}`,
			expectedFailure: vo.ContractOrderFailureVenueRefused},
		{name: "數量", statusCode: http.StatusBadRequest, body: `{"code":-4003,"msg":"Quantity less than or equal to zero."}`,
			expectedFailure: vo.ContractOrderFailureVenueRefused},
		{name: "幣安忙碌", statusCode: http.StatusServiceUnavailable, body: `oops`,
			expectedFailure: vo.ContractOrderFailureUncertain},
		{name: "結果不明", statusCode: http.StatusBadRequest, body: `{"code":-1007,"msg":"Timeout waiting for response from backend server. Send status unknown; execution status unknown."}`,
			expectedFailure: vo.ContractOrderFailureUncertain},
		{name: "沒見過的", statusCode: http.StatusBadRequest, body: `{"code":-9999,"msg":"Something new."}`,
			expectedFailure: vo.ContractOrderFailureOtherRefusal},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proxy, _ := contractVenueAnswering(t, testCase.statusCode, testCase.body)

			_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

			assert.Equal(t, testCase.expectedFailure, call.Failure)
		})
	}
}

func TestBinanceContractOrderProxyPassesTheVenuesOwnWordsOn(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusBadRequest,
		`{"code":-4164,"msg":"Order's notional must be no smaller than 5"}`)

	_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, "Order's notional must be no smaller than 5", call.ExchangeMessage)
}

func TestBinanceContractOrderProxyCallsAnUnansweredRequestUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(server.Close)
	proxy := exchange.NewBinanceContractOrderProxy(server.URL,
		&http.Client{Timeout: 20 * time.Millisecond}, aClockAt(t, verificationMoment))

	_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureUncertain, call.Failure)
}

func TestBinanceContractOrderProxyFindsAnOrderByItsClientID(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK,
		`{"status":"FILLED","executedQty":"0.002","avgPrice":"85000"}`)

	fill, call := proxy.FindMarketOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-41-open")

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.True(t, fill.ExecutedQuantity.Equal(decimal.RequireFromString("0.002")))
	assert.Equal(t, http.MethodGet, (*received)[0].method)
	assert.Equal(t, "gt-ao-41-open", (*received)[0].query.Get("origClientOrderId"))
}

func TestBinanceContractOrderProxyReportsAnOrderTheVenueHasNoRecordOf(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusBadRequest, `{"code":-2013,"msg":"Order does not exist."}`)

	_, call := proxy.FindMarketOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-41-open")

	assert.Equal(t, vo.ContractOrderFailureNotFound, call.Failure)
}

func TestBinanceContractOrderProxyTakesAContractAlreadyIsolatedAsDone(t *testing.T) {
	requestedPaths := []string{}
	requestedLeverage := ""
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, request *http.Request) {
			requestedPaths = append(requestedPaths, request.URL.Path)
			if request.URL.Path == "/fapi/v1/marginType" {
				writer.WriteHeader(http.StatusBadRequest)
				_, _ = writer.Write([]byte(`{"code":-4046,"msg":"No need to change margin type."}`))

				return
			}
			requestedLeverage = request.URL.Query().Get("leverage")
			_, _ = writer.Write([]byte(`{"leverage":3,"symbol":"BTCUSDT"}`))
		}))
	t.Cleanup(server.Close)
	proxy := exchange.NewBinanceContractOrderProxy(server.URL, server.Client(), aClockAt(t, verificationMoment))

	call := proxy.PrepareIsolatedLeverage(t.Context(), aTradingKeyCredential, "BTCUSDT", 3)

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.Equal(t, []string{"/fapi/v1/marginType", "/fapi/v1/leverage"}, requestedPaths)
	assert.Equal(t, "3", requestedLeverage)
}

func TestBinanceContractOrderProxyCallsAPositionInTheWayAnAccountSettingRefusal(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusBadRequest,
		`{"code":-4048,"msg":"Margin type cannot be changed if there exists position."}`)

	call := proxy.PrepareIsolatedLeverage(t.Context(), aTradingKeyCredential, "BTCUSDT", 3)

	assert.Equal(t, vo.ContractOrderFailureAccountSettingRefused, call.Failure)
	assert.Len(t, *received, 1)
}

func TestBinanceContractOrderProxyReadsTheAccountsPositionMode(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK, `{"dualSidePosition":true}`)

	isHedgeMode, call := proxy.ReadPositionMode(t.Context(), aTradingKeyCredential)

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.True(t, isHedgeMode)
	assert.Equal(t, "/fapi/v1/positionSide/dual", (*received)[0].path)
}

func TestBinanceContractOrderProxyReadsANetShortPosition(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusOK,
		`[{"symbol":"BTCUSDT","positionAmt":"-0.003","positionSide":"BOTH"}]`)

	position, call := proxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.True(t, position.ShortQuantity.Equal(decimal.RequireFromString("0.003")))
	assert.True(t, position.LongQuantity.IsZero())
}

func TestBinanceContractOrderProxyReadsNoRowsAsNothingHeld(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusOK, `[]`)

	position, call := proxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.True(t, position.LongQuantity.IsZero())
	assert.True(t, position.ShortQuantity.IsZero())
}

func TestBinanceContractOrderProxyTakesAProtectiveOrderAlreadyGoneAsCancelled(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusBadRequest, `{"code":-2011,"msg":"Unknown order sent."}`)

	call := proxy.CancelProtectiveOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-30-sl")

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	assert.Equal(t, http.MethodDelete, (*received)[0].method)
	assert.Equal(t, "gt-ao-30-sl", (*received)[0].query.Get("clientAlgoId"))
}

func TestBinanceContractOrderProxyPlacesAMarkPriceStopThatOnlyReduces(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK, `{"algoId":1,"clientAlgoId":"gt-ao-41-sl"}`)

	call := proxy.PlaceProtectiveOrder(t.Context(), aTradingKeyCredential, vo.ContractProtectiveOrderVo{
		Symbol: "BTCUSDT", Side: vo.ContractOrderSideSell, Kind: vo.ContractProtectiveOrderStopLoss,
		TriggerPrice: decimal.NewFromInt(83725), Quantity: decimal.RequireFromString("0.002"),
		ClientOrderID: "gt-ao-41-sl",
	})

	assert.Equal(t, vo.ContractOrderFailureNone, call.Failure)
	request := (*received)[0]
	assert.Equal(t, "/fapi/v1/algoOrder", request.path)
	assert.Equal(t, "CONDITIONAL", request.query.Get("algoType"))
	assert.Equal(t, "STOP_MARKET", request.query.Get("type"))
	assert.Equal(t, "83725", request.query.Get("triggerPrice"))
	assert.Equal(t, "MARK_PRICE", request.query.Get("workingType"))
	assert.Equal(t, "true", request.query.Get("reduceOnly"))
	assert.Equal(t, "gt-ao-41-sl", request.query.Get("clientAlgoId"))
}

func TestBinanceContractOrderProxyPlacesATakeProfitAsATakeProfitMarket(t *testing.T) {
	proxy, received := contractVenueAnswering(t, http.StatusOK, `{"algoId":2}`)

	proxy.PlaceProtectiveOrder(t.Context(), aTradingKeyCredential, vo.ContractProtectiveOrderVo{
		Symbol: "BTCUSDT", Side: vo.ContractOrderSideSell, Kind: vo.ContractProtectiveOrderTakeProfit,
		TriggerPrice: decimal.NewFromInt(87550), Quantity: decimal.RequireFromString("0.002"),
		ClientOrderID: "gt-ao-41-tp",
	})

	assert.Equal(t, "TAKE_PROFIT_MARKET", (*received)[0].query.Get("type"))
}

func TestBinanceContractOrderProxyTellsAWorkingOrderFromALapsedOne(t *testing.T) {
	testCases := []struct {
		name            string
		body            string
		expectedFailure vo.ContractOrderFailureVo
	}{
		{name: "還在處理", body: `{"status":"NEW","executedQty":"0","avgPrice":"0"}`,
			expectedFailure: vo.ContractOrderFailureUncertain},
		{name: "沒有成交就失效", body: `{"status":"EXPIRED","executedQty":"0","avgPrice":"0"}`,
			expectedFailure: vo.ContractOrderFailureVenueRefused},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proxy, _ := contractVenueAnswering(t, http.StatusOK, testCase.body)

			_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

			assert.Equal(t, testCase.expectedFailure, call.Failure)
		})
	}
}

func TestBinanceContractOrderProxyPassesAFailedReadOn(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusBadRequest, `{"code":-2015,"msg":"Invalid API-key"}`)

	_, modeCall := proxy.ReadPositionMode(t.Context(), aTradingKeyCredential)
	_, positionCall := proxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")
	_, findCall := proxy.FindMarketOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-41-open")

	assert.Equal(t, vo.ContractOrderFailureKeyRejected, modeCall.Failure)
	assert.Equal(t, vo.ContractOrderFailureKeyRejected, positionCall.Failure)
	assert.Equal(t, vo.ContractOrderFailureKeyRejected, findCall.Failure)
}

func TestBinanceContractOrderProxyCallsAnAnswerItCannotReadUncertain(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusOK, `not json`)

	_, modeCall := proxy.ReadPositionMode(t.Context(), aTradingKeyCredential)
	_, positionCall := proxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")
	_, orderCall := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureUncertain, modeCall.Failure)
	assert.Equal(t, vo.ContractOrderFailureUncertain, positionCall.Failure)
	assert.Equal(t, vo.ContractOrderFailureUncertain, orderCall.Failure)
}

func TestBinanceContractOrderProxyCallsFiguresItCannotReadUncertain(t *testing.T) {
	positionProxy, _ := contractVenueAnswering(t, http.StatusOK, `[{"positionAmt":"lots","positionSide":"BOTH"}]`)
	orderProxy, _ := contractVenueAnswering(t, http.StatusOK, `{"status":"FILLED","executedQty":"some","avgPrice":"85000"}`)

	_, positionCall := positionProxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")
	_, orderCall := orderProxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureUncertain, positionCall.Failure)
	assert.Equal(t, vo.ContractOrderFailureUncertain, orderCall.Failure)
}

func TestBinanceContractOrderProxyAddsUpAHedgeModePair(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusOK,
		`[{"positionAmt":"0.002","positionSide":"LONG"},{"positionAmt":"0.003","positionSide":"SHORT"}]`)

	position, _ := proxy.ReadPosition(t.Context(), aTradingKeyCredential, "BTCUSDT")

	assert.True(t, position.LongQuantity.Equal(decimal.RequireFromString("0.002")))
	assert.True(t, position.ShortQuantity.Equal(decimal.RequireFromString("0.003")))
}

func TestBinanceContractOrderProxyFindsAProtectiveOrderByItsClientID(t *testing.T) {
	foundProxy, received := contractVenueAnswering(t, http.StatusOK, `{"algoId":1,"algoStatus":"NEW"}`)
	missingProxy, _ := contractVenueAnswering(t, http.StatusBadRequest, `{"code":-2013,"msg":"Order does not exist."}`)

	found := foundProxy.FindProtectiveOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-41-sl")
	missing := missingProxy.FindProtectiveOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-41-sl")

	assert.Equal(t, vo.ContractOrderFailureNone, found.Failure)
	assert.Equal(t, http.MethodGet, (*received)[0].method)
	assert.Equal(t, "/fapi/v1/algoOrder", (*received)[0].path)
	assert.Equal(t, "gt-ao-41-sl", (*received)[0].query.Get("clientAlgoId"))
	assert.Equal(t, vo.ContractOrderFailureNotFound, missing.Failure)
}

func TestBinanceContractOrderProxyPassesAFailedCancelOn(t *testing.T) {
	proxy, _ := contractVenueAnswering(t, http.StatusServiceUnavailable, `busy`)

	call := proxy.CancelProtectiveOrder(t.Context(), aTradingKeyCredential, "BTCUSDT", "gt-ao-30-sl")

	assert.Equal(t, vo.ContractOrderFailureUncertain, call.Failure)
}

func TestBinanceContractOrderProxyReadsARefusalItCannotParse(t *testing.T) {
	testCases := []struct {
		name            string
		statusCode      int
		expectedFailure vo.ContractOrderFailureVo
		expectedMessage string
	}{
		{name: "未授權", statusCode: http.StatusUnauthorized, expectedFailure: vo.ContractOrderFailureKeyRejected},
		{name: "其他", statusCode: http.StatusBadRequest, expectedFailure: vo.ContractOrderFailureOtherRefusal,
			expectedMessage: "HTTP 400"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			proxy, _ := contractVenueAnswering(t, testCase.statusCode, `<html>nope</html>`)

			_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

			assert.Equal(t, testCase.expectedFailure, call.Failure)
			assert.Equal(t, testCase.expectedMessage, call.ExchangeMessage)
		})
	}
}

func TestBinanceContractOrderProxyRefusesARequestItCannotBuild(t *testing.T) {
	proxy := exchange.NewBinanceContractOrderProxy("http://bad host", http.DefaultClient, aClockAt(t, verificationMoment))

	_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureOtherRefusal, call.Failure)
}

func TestBinanceContractOrderProxyCallsAnAnswerCutShortUncertain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "100")
		_, _ = writer.Write([]byte(`{"status"`))
	}))
	t.Cleanup(server.Close)
	proxy := exchange.NewBinanceContractOrderProxy(server.URL, server.Client(), aClockAt(t, verificationMoment))

	_, call := proxy.PlaceMarketOrder(t.Context(), aTradingKeyCredential, aMarketBuy)

	assert.Equal(t, vo.ContractOrderFailureUncertain, call.Failure)
}

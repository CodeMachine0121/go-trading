package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	positionModePath  = "/fapi/v1/positionSide/dual"
	positionRiskPath  = "/fapi/v3/positionRisk"
	marginTypePath    = "/fapi/v1/marginType"
	leveragePath      = "/fapi/v1/leverage"
	contractOrderPath = "/fapi/v1/order"
	// algoOrderPath is where the venue keeps conditional orders such as stop-market and take-profit-market.
	algoOrderPath = "/fapi/v1/algoOrder"
)

// unreadableAnswer is an answer that came back but could not be read, so whether the call did anything is unknown.
var unreadableAnswer = vo.ContractOrderCallVo{
	Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "看不懂幣安的回覆"}

// BinanceContractOrderProxy trades USDT-margined perpetual contracts with signed requests; neither key string ever appears in what it returns.
type BinanceContractOrderProxy struct {
	apiBaseUrl string
	httpClient *http.Client
	clockProxy domaininterface.IClockProxy
}

func NewBinanceContractOrderProxy(
	apiBaseUrl string, httpClient *http.Client, clockProxy domaininterface.IClockProxy,
) *BinanceContractOrderProxy {
	return &BinanceContractOrderProxy{
		apiBaseUrl: strings.TrimRight(apiBaseUrl, "/"),
		httpClient: httpClient,
		clockProxy: clockProxy,
	}
}

func (contractOrderProxy *BinanceContractOrderProxy) ReadPositionMode(
	executionContext context.Context, credential vo.TradingKeyCredentialVo,
) (bool, vo.ContractOrderCallVo) {
	body, call := contractOrderProxy.send(executionContext, credential, http.MethodGet, positionModePath,
		url.Values{})
	if call.Failure != vo.ContractOrderFailureNone {
		return false, call
	}

	var positionMode binancePositionModeResponse
	if decodeError := json.Unmarshal(body, &positionMode); decodeError != nil {
		return false, unreadableAnswer
	}

	return positionMode.DualSidePosition, call
}

// ReadPosition adds up every row on each side, so a one-way net position and a hedge-mode pair read the same.
func (contractOrderProxy *BinanceContractOrderProxy) ReadPosition(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string,
) (vo.ContractExchangePositionVo, vo.ContractOrderCallVo) {
	body, call := contractOrderProxy.send(executionContext, credential, http.MethodGet, positionRiskPath,
		url.Values{"symbol": {symbol}})
	if call.Failure != vo.ContractOrderFailureNone {
		return vo.ContractExchangePositionVo{}, call
	}

	var positionRows []binancePositionRiskResponse
	if decodeError := json.Unmarshal(body, &positionRows); decodeError != nil {
		return vo.ContractExchangePositionVo{}, unreadableAnswer
	}

	position := vo.ContractExchangePositionVo{LongQuantity: decimal.Zero, ShortQuantity: decimal.Zero}
	for _, positionRow := range positionRows {
		amount, parseError := decimal.NewFromString(positionRow.PositionAmt)
		if parseError != nil {
			return vo.ContractExchangePositionVo{}, vo.ContractOrderCallVo{
				Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "看不懂幣安回覆的倉位"}
		}

		switch {
		case positionRow.PositionSide == "SHORT" || amount.IsNegative():
			position.ShortQuantity = position.ShortQuantity.Add(amount.Abs())
		default:
			position.LongQuantity = position.LongQuantity.Add(amount.Abs())
		}
	}

	return position, call
}

func (contractOrderProxy *BinanceContractOrderProxy) PrepareIsolatedLeverage(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, leverage int,
) vo.ContractOrderCallVo {
	_, marginTypeCall := contractOrderProxy.send(executionContext, credential, http.MethodPost, marginTypePath,
		url.Values{"symbol": {symbol}, "marginType": {"ISOLATED"}})
	if marginTypeCall.Failure != vo.ContractOrderFailureNone {
		return marginTypeCall
	}

	_, leverageCall := contractOrderProxy.send(executionContext, credential, http.MethodPost, leveragePath,
		url.Values{"symbol": {symbol}, "leverage": {strconv.Itoa(leverage)}})

	return leverageCall
}

func (contractOrderProxy *BinanceContractOrderProxy) PlaceMarketOrder(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, order vo.ContractMarketOrderVo,
) (vo.ContractOrderFillVo, vo.ContractOrderCallVo) {
	parameters := url.Values{
		"symbol":           {order.Symbol},
		"side":             {string(order.Side)},
		"type":             {"MARKET"},
		"quantity":         {order.Quantity.String()},
		"newClientOrderId": {order.ClientOrderID},
		"newOrderRespType": {"RESULT"},
	}
	if order.ReduceOnly {
		parameters.Set("reduceOnly", "true")
	}

	body, call := contractOrderProxy.send(executionContext, credential, http.MethodPost, contractOrderPath,
		parameters)
	if call.Failure != vo.ContractOrderFailureNone {
		return vo.ContractOrderFillVo{}, call
	}

	return contractOrderProxy.fillOf(body)
}

func (contractOrderProxy *BinanceContractOrderProxy) FindMarketOrder(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
) (vo.ContractOrderFillVo, vo.ContractOrderCallVo) {
	body, call := contractOrderProxy.send(executionContext, credential, http.MethodGet, contractOrderPath,
		url.Values{"symbol": {symbol}, "origClientOrderId": {clientOrderID}})
	if call.Failure != vo.ContractOrderFailureNone {
		return vo.ContractOrderFillVo{}, call
	}

	return contractOrderProxy.fillOf(body)
}

// PlaceProtectiveOrder triggers on the mark price, so a momentary spike in the last trade does not close the position.
func (contractOrderProxy *BinanceContractOrderProxy) PlaceProtectiveOrder(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, order vo.ContractProtectiveOrderVo,
) vo.ContractOrderCallVo {
	orderType := "STOP_MARKET"
	if order.Kind == vo.ContractProtectiveOrderTakeProfit {
		orderType = "TAKE_PROFIT_MARKET"
	}

	_, call := contractOrderProxy.send(executionContext, credential, http.MethodPost, algoOrderPath, url.Values{
		"algoType":     {"CONDITIONAL"},
		"symbol":       {order.Symbol},
		"side":         {string(order.Side)},
		"type":         {orderType},
		"quantity":     {order.Quantity.String()},
		"triggerPrice": {order.TriggerPrice.String()},
		"workingType":  {"MARK_PRICE"},
		"reduceOnly":   {"true"},
		"clientAlgoId": {order.ClientOrderID},
	})

	return call
}

func (contractOrderProxy *BinanceContractOrderProxy) FindProtectiveOrder(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
) vo.ContractOrderCallVo {
	_, call := contractOrderProxy.send(executionContext, credential, http.MethodGet, algoOrderPath,
		url.Values{"symbol": {symbol}, "clientAlgoId": {clientOrderID}})

	return call
}

func (contractOrderProxy *BinanceContractOrderProxy) CancelProtectiveOrder(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, symbol string, clientOrderID string,
) vo.ContractOrderCallVo {
	_, call := contractOrderProxy.send(executionContext, credential, http.MethodDelete, algoOrderPath,
		url.Values{"symbol": {symbol}, "clientAlgoId": {clientOrderID}})
	if call.Failure == vo.ContractOrderFailureNotFound {
		return vo.ContractOrderCallVo{}
	}

	return call
}

// fillOf reads an order that executed nothing as still working when it is open, and as refused when the venue let it lapse.
func (contractOrderProxy *BinanceContractOrderProxy) fillOf(body []byte) (vo.ContractOrderFillVo, vo.ContractOrderCallVo) {
	var orderResponse binanceContractOrderResponse
	if decodeError := json.Unmarshal(body, &orderResponse); decodeError != nil {
		return vo.ContractOrderFillVo{}, unreadableAnswer
	}

	executedQuantity, quantityError := decimal.NewFromString(orderResponse.ExecutedQty)
	averagePrice, priceError := decimal.NewFromString(orderResponse.AvgPrice)
	if quantityError != nil || priceError != nil {
		return vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{
			Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "看不懂幣安回覆的成交"}
	}

	if !executedQuantity.IsPositive() {
		if orderResponse.Status == "NEW" || orderResponse.Status == "PARTIALLY_FILLED" {
			return vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{
				Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "幣安還在處理這一筆"}
		}

		return vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{
			Failure:         vo.ContractOrderFailureVenueRefused,
			ExchangeMessage: fmt.Sprintf("幣安沒有成交這一筆（%s）", orderResponse.Status)}
	}

	return vo.ContractOrderFillVo{ExecutedQuantity: executedQuantity, AveragePrice: averagePrice}, vo.ContractOrderCallVo{}
}

// send signs the parameters into the query string, which the venue accepts for every method, and sorts whatever comes back by what can be done about it.
// A request that may have reached the venue without a clear answer is uncertain, never failed, since what it asked for may have happened.
func (contractOrderProxy *BinanceContractOrderProxy) send(
	executionContext context.Context, credential vo.TradingKeyCredentialVo, method string, path string,
	parameters url.Values,
) ([]byte, vo.ContractOrderCallVo) {
	parameters.Set("timestamp", strconv.FormatInt(contractOrderProxy.clockProxy.Now().UnixMilli(), 10))
	parameters.Set("recvWindow", strconv.FormatInt(signedRequestWindow.Milliseconds(), 10))
	unsignedQuery := parameters.Encode()
	signer := hmac.New(sha256.New, []byte(credential.SecretKey))
	signer.Write([]byte(unsignedQuery))
	signedQuery := unsignedQuery + "&signature=" + hex.EncodeToString(signer.Sum(nil))

	request, buildError := http.NewRequestWithContext(executionContext, method,
		contractOrderProxy.apiBaseUrl+path+"?"+signedQuery, nil)
	if buildError != nil {
		return nil, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureOtherRefusal, ExchangeMessage: "無法組出這個請求"}
	}
	request.Header.Set("X-MBX-APIKEY", credential.ApiKey)

	response, requestError := contractOrderProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "連不上幣安"}
	}
	defer func() { _ = response.Body.Close() }()

	body, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "幣安的回覆沒有讀完"}
	}

	if response.StatusCode >= http.StatusInternalServerError ||
		response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusTeapot {
		return nil, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: "幣安暫時無法處理"}
	}

	if response.StatusCode != http.StatusOK {
		return nil, contractOrderProxy.failureOf(response.StatusCode, body, path)
	}

	return body, vo.ContractOrderCallVo{}
}

func (contractOrderProxy *BinanceContractOrderProxy) failureOf(
	statusCode int, body []byte, path string,
) vo.ContractOrderCallVo {
	errorResponse := binanceErrorResponse{}
	if decodeError := json.Unmarshal(body, &errorResponse); decodeError != nil {
		if statusCode == http.StatusUnauthorized {
			return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected}
		}

		return vo.ContractOrderCallVo{
			Failure: vo.ContractOrderFailureOtherRefusal, ExchangeMessage: fmt.Sprintf("HTTP %d", statusCode)}
	}

	message := errorResponse.Message
	switch {
	case path == marginTypePath && errorResponse.Code == marginTypeAlreadySetCode:
		return vo.ContractOrderCallVo{}
	case errorResponse.BlamesTheKey() || statusCode == http.StatusUnauthorized:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected, ExchangeMessage: message}
	case insufficientBalanceCodes[errorResponse.Code]:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureInsufficientBalance, ExchangeMessage: message}
	case venueRefusedCodes[errorResponse.Code]:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureVenueRefused, ExchangeMessage: message}
	case accountSettingRefusedCodes[errorResponse.Code]:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureAccountSettingRefused, ExchangeMessage: message}
	case orderNotFoundCodes[errorResponse.Code]:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureNotFound, ExchangeMessage: message}
	case unknownOutcomeCodes[errorResponse.Code]:
		return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain, ExchangeMessage: message}
	}

	return vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureOtherRefusal, ExchangeMessage: message}
}

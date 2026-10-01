package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// signedRequestWindow is Binance's default recvWindow, stated explicitly.
const signedRequestWindow = 5 * time.Second

const apiRestrictionsPath = "/sapi/v1/account/apiRestrictions"

// BinanceTradingKeyVerificationProxy asks Binance which permissions a key has with a signed request; neither key string ever appears in an error.
type BinanceTradingKeyVerificationProxy struct {
	apiBaseUrl string
	httpClient *http.Client
	clockProxy domaininterface.IClockProxy
}

func NewBinanceTradingKeyVerificationProxy(
	apiBaseUrl string, httpClient *http.Client, clockProxy domaininterface.IClockProxy,
) *BinanceTradingKeyVerificationProxy {
	return &BinanceTradingKeyVerificationProxy{
		apiBaseUrl: strings.TrimRight(apiBaseUrl, "/"),
		httpClient: httpClient,
		clockProxy: clockProxy,
	}
}

// VerifyTradingKey maps every Binance outcome, including unrecognised ones, to a verification; it never returns an error because nothing it can fail on is the caller's to fix.
func (verificationProxy *BinanceTradingKeyVerificationProxy) VerifyTradingKey(
	executionContext context.Context, credential vo.TradingKeyCredentialVo,
) (vo.TradingKeyVerificationVo, error) {
	queryValues := url.Values{}
	queryValues.Set("timestamp", strconv.FormatInt(verificationProxy.clockProxy.Now().UnixMilli(), 10))
	queryValues.Set("recvWindow", strconv.FormatInt(signedRequestWindow.Milliseconds(), 10))
	unsignedQuery := queryValues.Encode()
	signer := hmac.New(sha256.New, []byte(credential.SecretKey))
	signer.Write([]byte(unsignedQuery))
	signedQuery := unsignedQuery + "&signature=" + hex.EncodeToString(signer.Sum(nil))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		verificationProxy.apiBaseUrl+apiRestrictionsPath+"?"+signedQuery, nil)
	if buildError != nil {
		return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, nil
	}
	request.Header.Set("X-MBX-APIKEY", credential.ApiKey)

	response, requestError := verificationProxy.httpClient.Do(request)
	if requestError != nil {
		timeoutError, isNetworkError := errors.AsType[net.Error](requestError)
		if errors.Is(requestError, context.DeadlineExceeded) || (isNetworkError && timeoutError.Timeout()) {
			return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut}, nil
		}

		return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, nil
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		errorResponse := binanceErrorResponse{}
		decodeError := json.NewDecoder(response.Body).Decode(&errorResponse)
		if (decodeError == nil && errorResponse.BlamesTheKey()) ||
			response.StatusCode == http.StatusUnauthorized {
			return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, nil
		}

		return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, nil
	}

	restrictions := binanceApiRestrictionsResponse{}
	if decodeError := json.NewDecoder(response.Body).Decode(&restrictions); decodeError != nil {
		timeoutError, isNetworkError := errors.AsType[net.Error](decodeError)
		if isNetworkError && timeoutError.Timeout() {
			return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut}, nil
		}

		return vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, nil
	}

	return vo.TradingKeyVerificationVo{
		FailureReason:          vo.TradingKeyVerificationFailureNone,
		SpotTradingEnabled:     restrictions.EnableSpotAndMarginTrading,
		ContractTradingEnabled: restrictions.EnableFutures,
	}, nil
}

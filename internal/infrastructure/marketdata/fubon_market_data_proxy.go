package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// fubonTradingSessions are the two boards this venue publishes candles under. It is
// told which one to answer about, and it will not answer about both at once.
//
// Both are asked, every time, rather than working out which one the window falls in.
// When a market trades is the domain's knowledge and it is already applied before a
// window reaches here — narrowing it again from this side would be the same rule
// written twice, and the copy that was not updated would keep answering.
var fubonTradingSessions = []string{"REGULAR", "AFTERHOURS"}

// FubonMarketDataProxy fetches Taiwan index futures K candles from Fubon.
//
// Two things the rest of the system must not know about this source stop here. Its
// candles are published one board at a time, so a stretch of time that spans the day
// board and the evening board is two requests and one answer. And the code being
// watched — "the futures" — is not a code this venue trades: it lists a different
// contract every month, so which one is asked for is resolved on every fetch.
//
// Candles come back under the standing code rather than the contract's, which is what
// makes the chart one continuous line across a roll. The jump at the roll is real in
// the sense that it is in the data, and it is not the market moving — anything reading
// this line for a signal has to know that.
type FubonMarketDataProxy struct {
	listedContracts        fubonListedContracts
	intradayCandlesBaseUrl string
	apiKey                 string
	httpClient             *http.Client
}

func NewFubonMarketDataProxy(
	productsUrl string,
	intradayCandlesBaseUrl string,
	apiKey string,
	clockProxy _interface.IClockProxy,
	requestTimeout time.Duration,
) *FubonMarketDataProxy {
	httpClient := &http.Client{Timeout: requestTimeout}

	return &FubonMarketDataProxy{
		listedContracts: fubonListedContracts{
			productsUrl: productsUrl,
			apiKey:      apiKey,
			clockProxy:  clockProxy,
			httpClient:  httpClient,
		},
		intradayCandlesBaseUrl: intradayCandlesBaseUrl,
		apiKey:                 apiKey,
		httpClient:             httpClient,
	}
}

// FetchKCandles returns every K candle this source holds inside the window, oldest
// first. A window the source has nothing for is an empty result, not a failure.
//
// A source that cannot say which contract is nearest **is** a failure, though: the
// alternative is storing nothing while reporting that all is well, on a market that
// might have been trading all evening.
func (fubonMarketDataProxy *FubonMarketDataProxy) FetchKCandles(
	executionContext context.Context, window vo.KCandleFetchWindowVo,
) ([]vo.MarketKCandleVo, error) {
	nearestContract, resolveError := fubonMarketDataProxy.listedContracts.nearestTo(
		executionContext, window.Symbol)
	if resolveError != nil {
		return nil, resolveError
	}

	marketKCandles := make([]vo.MarketKCandleVo, 0)
	for _, tradingSession := range fubonTradingSessions {
		sessionKCandles, fetchError := fubonMarketDataProxy.askSession(
			executionContext, nearestContract.Symbol, window.Symbol, tradingSession)
		if fetchError != nil {
			return nil, fetchError
		}

		for _, sessionKCandle := range sessionKCandles {
			if sessionKCandle.OpenTime.Before(window.StartTime) ||
				sessionKCandle.OpenTime.After(window.EndTime) {
				continue
			}
			marketKCandles = append(marketKCandles, sessionKCandle)
		}
	}

	// The two boards arrive as two answers, and the evening board's candles start
	// before the day board's on the calendar day that follows it. Ordering them here
	// is what lets everything above read one sequence and never ask which board a
	// candle came from.
	slices.SortStableFunc(marketKCandles, func(
		oneKCandle vo.MarketKCandleVo, otherKCandle vo.MarketKCandleVo,
	) int {
		return oneKCandle.OpenTime.Compare(otherKCandle.OpenTime)
	})

	return marketKCandles, nil
}

// askSession asks for one board's candles for one contract, and reports them under
// the standing code the caller watches.
func (fubonMarketDataProxy *FubonMarketDataProxy) askSession(
	executionContext context.Context,
	contractSymbol string,
	standingSymbol string,
	tradingSession string,
) ([]vo.MarketKCandleVo, error) {
	queryValues := url.Values{}
	queryValues.Set("timeframe", fugleTimeframe)
	queryValues.Set("session", tradingSession)

	requestUrl := fubonMarketDataProxy.intradayCandlesBaseUrl + "/" +
		url.PathEscape(contractSymbol) + "?" + queryValues.Encode()
	request, buildError := http.NewRequestWithContext(
		executionContext, http.MethodGet, requestUrl, nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", standingSymbol, buildError)
	}
	request.Header.Set(fugleApiKeyHeader, fubonMarketDataProxy.apiKey)

	response, requestError := fubonMarketDataProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach market source for %s: %w", standingSymbol, requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"market source answered %d for %s", response.StatusCode, standingSymbol)
	}

	// This venue answers in the same shape its stock feed does — it is the same
	// vendor's technology — so the candle is read by the wire type that already
	// describes it rather than by a second copy of the same struct tags.
	var candlesAnswer fugleCandlesAnswer
	if decodeError := json.NewDecoder(response.Body).Decode(&candlesAnswer); decodeError != nil {
		return nil, fmt.Errorf("read k candles for %s: %w", standingSymbol, decodeError)
	}

	marketKCandles := make([]vo.MarketKCandleVo, 0, len(candlesAnswer.Data))
	for _, reportedCandle := range candlesAnswer.Data {
		marketKCandle, convertError := reportedCandle.toMarketKCandleVo(standingSymbol)
		if convertError != nil {
			return nil, convertError
		}
		marketKCandles = append(marketKCandles, marketKCandle)
	}

	return marketKCandles, nil
}

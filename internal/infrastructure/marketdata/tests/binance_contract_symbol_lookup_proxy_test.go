package marketdata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wholeContractCatalogue is what the contract venue answers with whatever it is asked
// about: the entire listing, and an ordinary success even for a pair that does not
// exist. Recognising a typo therefore means scanning it — and the listing carries
// more than names, which is what lets the scan also refuse a contract nobody could
// follow.
//
// The four entries are the four shapes the venue actually publishes: a perpetual, a
// traditional-finance perpetual, one it has stopped trading, and a dated future.
const wholeContractCatalogue = `{"symbols":[
  {"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL"},
  {"symbol":"1000PEPEUSDT","status":"TRADING","contractType":"PERPETUAL"},
  {"symbol":"XAUUSDT","status":"TRADING","contractType":"TRADIFI_PERPETUAL"},
  {"symbol":"OMGUSDT","status":"SETTLING","contractType":"PERPETUAL"},
  {"symbol":"BTCUSDT_260925","status":"TRADING","contractType":"CURRENT_QUARTER"}
]}`

func TestContractSymbolLookupFindsAListedContract(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupFindsAContractThatOnlyThisVenueHas(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "1000PEPEUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupRejectsAnUnlistedContractDespiteASuccessfulAnswer(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "NOSUCHPAIR")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

func TestContractSymbolLookupDoesNotTreatALookAlikeNameAsAMatch(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, `{"symbols":[{"symbol":"1000SHIBUSDT","status":"TRADING","contractType":"PERPETUAL"}]}`),
		requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "SHIBUSDT")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

func TestContractSymbolLookupSaysSoWhenTheSourceRefuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusTooManyRequests)
		}))
	t.Cleanup(server.Close)
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		server.URL, requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, lookupError, "429")
}

func TestContractSymbolLookupSaysSoWhenTheAnswerCannotBeRead(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, "not json"), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.Error(t, lookupError)
}

func TestContractSymbolLookupSaysSoWhenTheSourceCannotBeReached(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		"http://127.0.0.1:1", requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.Error(t, lookupError)
}

func TestContractSymbolLookupGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), requestTimeout, marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, lookupError := lookupProxy.LookUpSymbol(abandonedContext, "BTCUSDT")

	assert.ErrorIs(t, lookupError, context.Canceled)
}

func TestContractSymbolLookupSaysSoWhenTheAddressCannotBeTurnedIntoARequest(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		"http://\x7f", requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, lookupError, "reach contract market source")
}

func TestContractSymbolLookupAcceptsAPerpetualOnATraditionalFinanceUnderlying(t *testing.T) {
	// Gold, silver and a share are listed as perpetuals here — they carry the venue's
	// no-delivery date exactly as the crypto ones do, so there is nothing about them
	// this system cannot follow.
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "XAUUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupRefusesAContractNobodyCouldFollow(t *testing.T) {
	// Being in the catalogue is not enough. A contract the venue has stopped trading
	// answers every fetch with nothing, so following one would put a symbol on the
	// watchlist that is fetched every minute forever and stores nothing — which on
	// this venue reads exactly like a quiet market, because a perpetual contract is
	// never presumed shut. A dated future does the same on its delivery day.
	testCases := []struct {
		name   string
		symbol string
	}{
		{name: "已經停止交易的合約", symbol: "OMGUSDT"},
		{name: "有交割日的季度合約", symbol: "BTCUSDT_260925"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
				servedBy(t, wholeContractCatalogue), requestTimeout, unpaced())

			listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), testCase.symbol)

			require.NoError(t, lookupError)
			assert.False(t, listing.IsListed)
		})
	}
}

func TestContractSymbolLookupRefusesAKindItDoesNotRecognise(t *testing.T) {
	// Refusing a new perpetual kind is a sentence somebody reads and one line to fix;
	// following a new dated kind is a contract that silently stops producing candles
	// on its delivery day. So an unrecognised kind is refused, not assumed.
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, `{"symbols":[{"symbol":"NEWUSDT","status":"TRADING","contractType":"SOMETHING_NEW"}]}`),
		requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "NEWUSDT")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

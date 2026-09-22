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

// wholeContractCatalogue is what the contract venue answers with whatever it is
// asked about: the entire listing, and an ordinary success even for a pair that does
// not exist. Recognising a typo therefore means scanning it.
const wholeContractCatalogue = `{"symbols":[{"symbol":"BTCUSDT"},{"symbol":"1000PEPEUSDT"}]}`

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
		servedBy(t, `{"symbols":[{"symbol":"1000SHIBUSDT"}]}`), requestTimeout, unpaced())

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

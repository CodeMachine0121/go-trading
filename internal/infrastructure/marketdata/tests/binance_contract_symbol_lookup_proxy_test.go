package marketdata_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/shopspring/decimal"
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

// noFundingIntervals is a funding interval list that names no contract: every
// contract settles on the venue's default.
func noFundingIntervals(t *testing.T) string {
	t.Helper()

	return servedBy(t, "[]")
}

func TestContractSymbolLookupFindsAListedContract(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupFindsAContractThatOnlyThisVenueHas(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "1000PEPEUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupRejectsAnUnlistedContractDespiteASuccessfulAnswer(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "NOSUCHPAIR")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

func TestContractSymbolLookupDoesNotTreatALookAlikeNameAsAMatch(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, `{"symbols":[{"symbol":"1000SHIBUSDT","status":"TRADING","contractType":"PERPETUAL"}]}`),
		noFundingIntervals(t), requestTimeout, unpaced())

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
		server.URL, noFundingIntervals(t), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, lookupError, "429")
}

func TestContractSymbolLookupSaysSoWhenTheAnswerCannotBeRead(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, "not json"), noFundingIntervals(t), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.Error(t, lookupError)
}

func TestContractSymbolLookupSaysSoWhenTheSourceCannotBeReached(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		"http://127.0.0.1:1", noFundingIntervals(t), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.Error(t, lookupError)
}

func TestContractSymbolLookupGivesUpWhenTheCallerHasGoneAway(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, marketdata.NewRequestPacer(1))
	abandonedContext, abandon := context.WithCancel(t.Context())
	abandon()

	_, lookupError := lookupProxy.LookUpSymbol(abandonedContext, "BTCUSDT")

	assert.ErrorIs(t, lookupError, context.Canceled)
}

func TestContractSymbolLookupSaysSoWhenTheAddressCannotBeTurnedIntoARequest(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		"http://\x7f", noFundingIntervals(t), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, lookupError, "reach contract market source")
}

func TestContractSymbolLookupAcceptsAPerpetualOnATraditionalFinanceUnderlying(t *testing.T) {
	// Gold, silver and a share are listed as perpetuals here — they carry the venue's
	// no-delivery date exactly as the crypto ones do, so there is nothing about them
	// this system cannot follow.
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

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
				servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

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
		noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "NEWUSDT")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

// catalogueWithSpecifications is the catalogue as the venue actually spells a
// specification: filters of several kinds, each carrying its own fields, and the
// maintenance margin written as a percentage.
const catalogueWithSpecifications = `{"symbols":[
  {"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL",
   "maintMarginPercent":"2.5000","liquidationFee":"0.012500",
   "filters":[
     {"filterType":"PRICE_FILTER","tickSize":"0.10","minPrice":"556.80","maxPrice":"4529764"},
     {"filterType":"LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"1000"},
     {"filterType":"MARKET_LOT_SIZE","stepSize":"0.001","minQty":"0.001","maxQty":"120"},
     {"filterType":"MIN_NOTIONAL","notional":"50"}]},
  {"symbol":"LPTUSDT","status":"TRADING","contractType":"PERPETUAL",
   "maintMarginPercent":"1.0000","liquidationFee":"0.015000",
   "filters":[
     {"filterType":"PRICE_FILTER","tickSize":"0.001"},
     {"filterType":"LOT_SIZE","stepSize":"0.1","minQty":"0.3"},
     {"filterType":"MIN_NOTIONAL","notional":"5"}]},
  {"symbol":"ODDUSDT","status":"TRADING","contractType":"PERPETUAL",
   "maintMarginPercent":"","liquidationFee":"0.01","filters":[]},
  {"symbol":"OMGUSDT","status":"SETTLING","contractType":"PERPETUAL",
   "maintMarginPercent":"2.5000","liquidationFee":"0.012500",
   "filters":[
     {"filterType":"PRICE_FILTER","tickSize":"0.0001"},
     {"filterType":"LOT_SIZE","stepSize":"1","minQty":"1"},
     {"filterType":"MIN_NOTIONAL","notional":"5"}]}
]}`

const fundingIntervalList = `[
  {"symbol":"LPTUSDT","adjustedFundingRateCap":"0.02","adjustedFundingRateFloor":"-0.02","fundingIntervalHours":4,"disclaimer":false}
]`

func TestContractSymbolLookupCarriesTheSpecificationOfTheContractItFinds(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, catalogueWithSpecifications), servedBy(t, fundingIntervalList), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	specification := listing.Specification
	assert.Equal(t, "BTCUSDT", specification.Symbol)
	assert.True(t, decimal.RequireFromString("0.1").Equal(specification.TickSize))
	assert.True(t, decimal.RequireFromString("0.001").Equal(specification.QuantityStep))
	assert.True(t, decimal.RequireFromString("0.001").Equal(specification.MinimumQuantity))
	assert.True(t, decimal.RequireFromString("50").Equal(specification.MinimumNotional))
	assert.True(t, decimal.RequireFromString("0.025").Equal(specification.MaintenanceMarginRate))
	assert.True(t, decimal.RequireFromString("0.0125").Equal(specification.LiquidationFeeRate))
	assert.Nil(t, specification.FundingIntervalHours)
}

func TestContractSymbolLookupReadsTheFundingIntervalFromTheListOfExceptions(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, catalogueWithSpecifications), servedBy(t, fundingIntervalList), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "LPTUSDT")

	require.NoError(t, lookupError)
	require.NotNil(t, listing.Specification.FundingIntervalHours)
	assert.Equal(t, 4, *listing.Specification.FundingIntervalHours)
}

func TestContractSymbolLookupStillFollowsAContractWhoseSpecificationCannotBeRead(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, catalogueWithSpecifications), servedBy(t, fundingIntervalList), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "ODDUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	assert.True(t, listing.Specification.TickSize.IsZero())
}

func TestContractSymbolLookupStillFollowsAContractWhenTheFundingIntervalsCannotBeRead(t *testing.T) {
	// The catalogue already said the contract can be followed. Without the interval
	// list its specification cannot be complete, so none is handed on — rather than a
	// guessed eight hours — and the add goes ahead.
	for _, fundingIntervalsUrl := range []string{servedBy(t, "not json"), "http://127.0.0.1:1"} {
		lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
			servedBy(t, catalogueWithSpecifications), fundingIntervalsUrl, requestTimeout, unpaced())

		listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

		require.NoError(t, lookupError)
		assert.True(t, listing.IsListed)
		assert.True(t, listing.Specification.TickSize.IsZero())
		assert.Nil(t, listing.Specification.FundingIntervalHours)
	}
}

func TestContractSymbolLookupRefreshesNothingWhenTheFundingIntervalsCannotBeRead(t *testing.T) {
	// A refresh that cannot finish changes nothing: every contract keeps what it was
	// last confirmed with.
	for _, fundingIntervalsUrl := range []string{servedBy(t, "not json"), "http://127.0.0.1:1"} {
		lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
			servedBy(t, catalogueWithSpecifications), fundingIntervalsUrl, requestTimeout, unpaced())

		specifications, fetchError := lookupProxy.FetchTradingSpecifications(t.Context())

		assert.Error(t, fetchError)
		assert.Nil(t, specifications)
	}
}

func TestContractSymbolLookupReportsTheSpecificationOfEveryFollowableContract(t *testing.T) {
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, catalogueWithSpecifications), servedBy(t, fundingIntervalList), requestTimeout, unpaced())

	specifications, fetchError := lookupProxy.FetchTradingSpecifications(t.Context())

	require.NoError(t, fetchError)
	// ODDUSDT cannot be read and OMGUSDT is no longer trading: neither is reported.
	require.Len(t, specifications, 2)
	assert.Equal(t, "BTCUSDT", specifications[0].Symbol)
	assert.Nil(t, specifications[0].FundingIntervalHours)
	assert.Equal(t, "LPTUSDT", specifications[1].Symbol)
	assert.True(t, decimal.RequireFromString("0.01").Equal(specifications[1].MaintenanceMarginRate))
	assert.True(t, decimal.RequireFromString("0.1").Equal(specifications[1].QuantityStep))
	assert.True(t, decimal.RequireFromString("0.3").Equal(specifications[1].MinimumQuantity))
	require.NotNil(t, specifications[1].FundingIntervalHours)
	assert.Equal(t, 4, *specifications[1].FundingIntervalHours)
}

func TestContractSymbolLookupSaysSoWhenTheWholeCatalogueCannotBeFetched(t *testing.T) {
	for _, catalogueUrl := range []string{servedBy(t, "not json"), "http://127.0.0.1:1"} {
		lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
			catalogueUrl, servedBy(t, fundingIntervalList), requestTimeout, unpaced())

		specifications, fetchError := lookupProxy.FetchTradingSpecifications(t.Context())

		assert.Error(t, fetchError)
		assert.Nil(t, specifications)
	}
}

func TestContractSymbolLookupSaysSoWhenTheCatalogueIsCutOff(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Length", "100")
		_, _ = writer.Write([]byte("{"))
	}))
	t.Cleanup(server.Close)
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		server.URL, noFundingIntervals(t), requestTimeout, unpaced())

	_, lookupError := lookupProxy.LookUpSymbol(t.Context(), "BTCUSDT")

	assert.ErrorContains(t, lookupError, "read contract market source answer")
}

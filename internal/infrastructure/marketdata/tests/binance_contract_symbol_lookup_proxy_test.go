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

// wholeContractCatalogue is returned for any query, covering a perpetual, a traditional-finance perpetual, a delisted contract and a dated future.
const wholeContractCatalogue = `{"symbols":[
  {"symbol":"BTCUSDT","status":"TRADING","contractType":"PERPETUAL"},
  {"symbol":"1000PEPEUSDT","status":"TRADING","contractType":"PERPETUAL"},
  {"symbol":"XAUUSDT","status":"TRADING","contractType":"TRADIFI_PERPETUAL"},
  {"symbol":"OMGUSDT","status":"SETTLING","contractType":"PERPETUAL"},
  {"symbol":"BTCUSDT_260925","status":"TRADING","contractType":"CURRENT_QUARTER"}
]}`

// noFundingIntervals names no contract, so every contract uses the venue default.
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
	// Gold, silver and share perpetuals carry the same no-delivery date as crypto ones and are followable.
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, wholeContractCatalogue), noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "XAUUSDT")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
}

func TestContractSymbolLookupRefusesAContractNobodyCouldFollow(t *testing.T) {
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
	lookupProxy := marketdata.NewBinanceContractSymbolLookupProxy(
		servedBy(t, `{"symbols":[{"symbol":"NEWUSDT","status":"TRADING","contractType":"SOMETHING_NEW"}]}`),
		noFundingIntervals(t), requestTimeout, unpaced())

	listing, lookupError := lookupProxy.LookUpSymbol(t.Context(), "NEWUSDT")

	require.NoError(t, lookupError)
	assert.False(t, listing.IsListed)
}

// catalogueWithSpecifications has filters of several kinds and the maintenance margin as a percentage.
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
	// Without the interval list no specification is returned rather than guessing eight hours, but the add still succeeds.
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
	// A refresh that cannot finish changes nothing.
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
	// ODDUSDT is unreadable and OMGUSDT is no longer trading, so neither is reported.
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

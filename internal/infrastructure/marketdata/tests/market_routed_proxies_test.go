package marketdata_test

import (
	"testing"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestFetchingIsSentToTheSourceThatServesTheWindowsMarket(t *testing.T) {
	mockController := gomock.NewController(t)
	taiwanSource := mocks.NewMockIMarketDataProxy(mockController)
	cryptoSource := mocks.NewMockIMarketDataProxy(mockController)
	window := vo.NewKCandleFetchWindowVo(
		"2330", vo.MarketTaiwanStock, at(9, 0), at(9, 20))
	// No expectation on the other source, so asking it fails the test.
	taiwanSource.EXPECT().FetchKCandles(gomock.Any(), window).
		Return([]vo.MarketKCandleVo{{Symbol: "2330"}}, nil)

	marketKCandles, fetchError := marketdata.NewMarketRoutedMarketDataProxy(
		map[vo.MarketVo]_interface.IMarketDataProxy{
			vo.MarketTaiwanStock: taiwanSource,
			vo.MarketCrypto:      cryptoSource,
		}).FetchKCandles(t.Context(), window)

	require.NoError(t, fetchError)
	require.Len(t, marketKCandles, 1)
	assert.Equal(t, "2330", marketKCandles[0].Symbol)
}

func TestFetchingForAMarketWithNoSourceIsAFailureRatherThanNothing(t *testing.T) {
	marketKCandles, fetchError := marketdata.NewMarketRoutedMarketDataProxy(
		map[vo.MarketVo]_interface.IMarketDataProxy{}).
		FetchKCandles(t.Context(), vo.NewKCandleFetchWindowVo(
			"2330", vo.MarketTaiwanStock, at(9, 0), at(9, 20)))

	require.Error(t, fetchError)
	assert.Contains(t, fetchError.Error(), "taiwanStock")
	assert.Empty(t, marketKCandles)
}

func TestFollowingIsSentToTheSourceThatServesTheTargetsMarket(t *testing.T) {
	mockController := gomock.NewController(t)
	taiwanSource := mocks.NewMockILiveMarketDataProxy(mockController)
	cryptoSource := mocks.NewMockILiveMarketDataProxy(mockController)
	target := vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"})
	feed := make(chan vo.LiveKCandleVo)
	taiwanSource.EXPECT().FollowKCandles(gomock.Any(), target).Return(feed, nil)

	liveKCandles, followError := marketdata.NewMarketRoutedLiveMarketDataProxy(
		map[vo.MarketVo]_interface.ILiveMarketDataProxy{
			vo.MarketTaiwanStock: taiwanSource,
			vo.MarketCrypto:      cryptoSource,
		}).FollowKCandles(t.Context(), target)

	require.NoError(t, followError)
	assert.NotNil(t, liveKCandles)
}

func TestFollowingAMarketWithNoSourceIsAFailure(t *testing.T) {
	_, followError := marketdata.NewMarketRoutedLiveMarketDataProxy(
		map[vo.MarketVo]_interface.ILiveMarketDataProxy{}).
		FollowKCandles(t.Context(), vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}))

	require.Error(t, followError)
	assert.Contains(t, followError.Error(), "taiwanStock")
}

func TestALookupIsSentToTheSourceThatServesThatMarket(t *testing.T) {
	mockController := gomock.NewController(t)
	taiwanSource := mocks.NewMockISymbolLookupProxy(mockController)
	cryptoSource := mocks.NewMockISymbolLookupProxy(mockController)
	taiwanSource.EXPECT().LookUpSymbol(gomock.Any(), vo.MarketTaiwanStock, "2330").
		Return(vo.SymbolListingVo{IsListed: true, DisplayName: "台積電"}, nil)

	listing, lookupError := marketdata.NewMarketRoutedSymbolLookupProxy(
		map[vo.MarketVo]_interface.ISymbolLookupProxy{
			vo.MarketTaiwanStock: taiwanSource,
			vo.MarketCrypto:      cryptoSource,
		}).LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.NoError(t, lookupError)
	assert.True(t, listing.IsListed)
	// Routing never alters what the source answered.
	assert.Equal(t, "台積電", listing.DisplayName)
}

func TestALookupForAMarketWithNoSourceIsAFailureRatherThanANo(t *testing.T) {
	listing, lookupError := marketdata.NewMarketRoutedSymbolLookupProxy(
		map[vo.MarketVo]_interface.ISymbolLookupProxy{}).
		LookUpSymbol(t.Context(), vo.MarketTaiwanStock, "2330")

	require.Error(t, lookupError)
	assert.False(t, listing.IsListed)
}

func TestRoutingWorksFromItsOwnCopyOfTheSources(t *testing.T) {
	// The map is copied, so a later change by its owner does not repoint routing.
	mockController := gomock.NewController(t)
	taiwanSource := mocks.NewMockIMarketDataProxy(mockController)
	window := vo.NewKCandleFetchWindowVo("2330", vo.MarketTaiwanStock, at(9, 0), at(9, 20))
	taiwanSource.EXPECT().FetchKCandles(gomock.Any(), window).
		Return([]vo.MarketKCandleVo{}, nil)

	sources := map[vo.MarketVo]_interface.IMarketDataProxy{vo.MarketTaiwanStock: taiwanSource}
	marketRoutedMarketDataProxy := marketdata.NewMarketRoutedMarketDataProxy(sources)
	delete(sources, vo.MarketTaiwanStock)

	_, fetchError := marketRoutedMarketDataProxy.FetchKCandles(t.Context(), window)

	require.NoError(t, fetchError)
}

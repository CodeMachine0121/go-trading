package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rosterAt is a Taipei moment inside the Taiwan session, which is when a roster
// holds any places at all.
func rosterAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

func watchedIn(market vo.MarketVo, symbols ...string) []entities.TradingSymbol {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(market), IsWatched: true,
		})
	}

	return watchedSymbols
}

func channelsOf(t *testing.T, watchedSymbols []entities.TradingSymbol) []vo.LiveFollowChannelVo {
	t.Helper()

	return domains.NewLiveFollowRosterDomain(
		watchedSymbols, marketCatalog(), rosterAt(t, "2026-09-08T10:00:00+08:00")).
		Channels(marketCatalog())
}

// The whole point: the plan allows one line carrying five, so four place holders are
// four symbols on one line — not four lines.
func TestALimitedMarketPutsItsWholeRosterOnOneChannel(t *testing.T) {
	channels := channelsOf(t, watchedIn(vo.MarketTaiwanStock, "2330", "2454", "2603", "2609"))

	require.Len(t, channels, 1)
	assert.Equal(t, vo.MarketTaiwanStock, channels[0].Market)
	assert.Equal(t, []string{"2330", "2454", "2603", "2609"}, channels[0].Symbols)
}

// One symbol is not a different case. It goes down a line of its own kind, so
// nothing anywhere has to ask how many there are before deciding what to do.
func TestASingleHolderStillTravelsOnAChannel(t *testing.T) {
	channels := channelsOf(t, watchedIn(vo.MarketTaiwanStock, "2330"))

	require.Len(t, channels, 1)
	assert.Equal(t, []string{"2330"}, channels[0].Symbols)
}

// Out of hours a market holds no places, and a line with nothing on it is a line
// nobody should be paying for.
func TestAnEmptyRosterAsksForNoChannels(t *testing.T) {
	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketTaiwanStock, "2330"), marketCatalog(),
		rosterAt(t, "2026-09-08T20:00:00+08:00"))

	assert.Empty(t, rosterDomain.Channels(marketCatalog()))
}

// A market with no ceiling holds no places, so the roster asks for no channels of
// it at all — its lines are opened by whoever looks at a symbol, one symbol each.
// That is the difference between "no place for you" and "nobody has asked yet".
func TestAMarketWithNoCeilingIsNotRosteredIntoChannels(t *testing.T) {
	assert.Empty(t, channelsOf(t, watchedIn(vo.MarketCrypto, "BTCUSDT", "ETHUSDT")))
}

// More holders than one line carries is what the second number is for. Six symbols
// at three to a line is two lines, and the plan is respected without anybody
// counting.
func TestMoreHoldersThanOneChannelCarriesAreCutIntoSeveral(t *testing.T) {
	marketCatalogDomain := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {SimultaneousChannelCeiling: 2, SymbolsPerLiveChannel: 3},
	})

	channels := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketCrypto, "AAA", "BBB", "CCC", "DDD", "EEE", "FFF"),
		marketCatalogDomain, rosterAt(t, "2026-09-08T10:00:00+08:00")).
		Channels(marketCatalogDomain)

	require.Len(t, channels, 2)
	assert.Equal(t, []string{"AAA", "BBB", "CCC"}, channels[0].Symbols)
	assert.Equal(t, []string{"DDD", "EEE", "FFF"}, channels[1].Symbols)
}

// The key is the whole point of the type: the same set is the same channel however
// it was spelled, and a different set is a different channel. Everything that
// rebuilds a channel when its symbols change, and leaves it alone when they do not,
// rests on exactly this.
func TestAChannelIsTheSameChannelWhateverOrderItsSymbolsArriveIn(t *testing.T) {
	first := vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2454", "2330"})
	second := vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330", "2454"})

	assert.Equal(t, second.Key, first.Key)
	assert.Equal(t, []string{"2330", "2454"}, first.Symbols)
}

func TestAddingOrRemovingASymbolMakesItADifferentChannel(t *testing.T) {
	twoSymbols := vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330", "2454"})

	assert.NotEqual(t, twoSymbols.Key,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330", "2454", "2603"}).Key)
	assert.NotEqual(t, twoSymbols.Key,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}).Key)
}

// Two markets following the same symbol are two channels. The market is part of
// what a channel is, not a detail carried alongside.
func TestTheSameSymbolOnTwoMarketsIsTwoChannels(t *testing.T) {
	assert.NotEqual(t,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}).Key,
		vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"2330"}).Key)
}

// Sorting must not reach back into what the caller handed over — a roster that found
// its own list reordered underneath it would be a very quiet kind of wrong.
func TestBuildingAChannelLeavesTheCallersListAlone(t *testing.T) {
	symbols := []string{"2454", "2330"}

	vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, symbols)

	assert.Equal(t, []string{"2454", "2330"}, symbols)
}

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
		Channels()
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

	assert.Empty(t, rosterDomain.Channels())
}

// A market followed by whoever looks holds no places, so the roster asks for no
// channels of it at all — its lines are opened when somebody opens a chart, one
// symbol each. That is the difference between "no place for you" and "nobody has
// asked yet".
func TestAMarketFollowedByWhoeverLooksIsNotRosteredIntoChannels(t *testing.T) {
	assert.Empty(t, channelsOf(t, watchedIn(vo.MarketCrypto, "BTCUSDT", "ETHUSDT")))
}

// More holders than one line carries is what the second number is for. Six symbols
// at three to a line is two lines, and the plan is respected without anybody
// counting.
func TestMoreHoldersThanOneChannelCarriesAreCutIntoSeveral(t *testing.T) {
	marketCatalogDomain := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {
			FollowsFixedRoster:         true,
			SimultaneousChannelCeiling: 2,
			SymbolsPerLiveChannel:      3,
		},
	})

	channels := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketCrypto, "AAA", "BBB", "CCC", "DDD", "EEE", "FFF"),
		marketCatalogDomain, rosterAt(t, "2026-09-08T10:00:00+08:00")).
		Channels()

	require.Len(t, channels, 2)
	assert.Equal(t, []string{"AAA", "BBB", "CCC"}, channels[0].Symbols)
	assert.Equal(t, []string{"DDD", "EEE", "FFF"}, channels[1].Symbols)
}

// rosteredUncappedCatalog is a market followed from a roster whose source puts no
// limit on how many may be followed at once — the shape a venue takes when its
// quotes are asked for rather than subscribed to.
func rosteredUncappedCatalog(symbolsPerChannel int) domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketTaiwanStock: {
			TradingSession:        taiwanStockRules().TradingSession,
			FollowsFixedRoster:    true,
			SymbolsPerLiveChannel: symbolsPerChannel,
		},
	})
}

// The point of the whole slice: with nothing capping it, every watched symbol is on
// the roster. An uncapped rostered market is not an empty roster — reading it that
// way would stop following a market at the moment its source stopped limiting it.
func TestAnUncappedRosteredMarketHoldsEveryWatchedSymbol(t *testing.T) {
	watchedSymbols := watchedIn(vo.MarketTaiwanStock,
		"1101", "1301", "2317", "2330", "2454", "2603", "2609", "2881")

	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedSymbols, rosteredUncappedCatalog(50),
		rosterAt(t, "2026-09-08T10:00:00+08:00"))

	assert.Len(t, rosterDomain.Symbols(), 8)
	for _, watchedSymbol := range watchedSymbols {
		assert.True(t, rosterDomain.Holds(watchedSymbol.Symbol), watchedSymbol.Symbol)
	}
}

// One is not a different case, and neither is none.
func TestAnUncappedRosteredMarketHandlesOneAndNone(t *testing.T) {
	testCases := []struct {
		name          string
		watchedSymbol []string
		channelCount  int
	}{
		{name: "one watched stock", watchedSymbol: []string{"2330"}, channelCount: 1},
		{name: "nothing watched", watchedSymbol: nil, channelCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			channels := domains.NewLiveFollowRosterDomain(
				watchedIn(vo.MarketTaiwanStock, testCase.watchedSymbol...),
				rosteredUncappedCatalog(50), rosterAt(t, "2026-09-08T10:00:00+08:00")).
				Channels()

			assert.Len(t, channels, testCase.channelCount)
		})
	}
}

// More watched symbols than one round of questions covers is cut into several, and
// every one of them is still followed. Nobody looking at a chart can tell which
// round theirs travelled in.
func TestAnUncappedRosterIsCutIntoChannelsWithoutDroppingAnySymbol(t *testing.T) {
	channels := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketTaiwanStock,
			"1101", "1301", "2317", "2330", "2454", "2603", "2609", "2881", "3711"),
		rosteredUncappedCatalog(4), rosterAt(t, "2026-09-08T10:00:00+08:00")).
		Channels()

	require.Len(t, channels, 3)

	followedSymbols := make([]string, 0, 9)
	for _, channel := range channels {
		assert.LessOrEqual(t, len(channel.Symbols), 4)
		followedSymbols = append(followedSymbols, channel.Symbols...)
	}

	assert.ElementsMatch(t, []string{
		"1101", "1301", "2317", "2330", "2454", "2603", "2609", "2881", "3711",
	}, followedSymbols)
}

// Out of hours an uncapped roster is as empty as a capped one. Nothing to follow is
// nothing to follow, whatever the source would have allowed.
func TestAnUncappedRosteredMarketHoldsNothingOutOfHours(t *testing.T) {
	channels := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketTaiwanStock, "2330", "2454"),
		rosteredUncappedCatalog(50), rosterAt(t, "2026-09-08T20:00:00+08:00")).
		Channels()

	assert.Empty(t, channels)
}

// What a console is told before somebody picks a symbol. An uncapped rostered market
// still follows only what is watched, so saying otherwise would hand somebody a
// chart that looks live and never moves.
func TestHasLiveUpdatesAsksWhetherTheMarketIsRosteredNotWhetherItIsCapped(t *testing.T) {
	uncappedCatalog := rosteredUncappedCatalog(50)
	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedIn(vo.MarketTaiwanStock, "2330"), uncappedCatalog,
		rosterAt(t, "2026-09-08T10:00:00+08:00"))

	testCases := []struct {
		name           string
		symbol         string
		market         vo.MarketVo
		catalog        domains.MarketCatalogDomain
		hasLiveUpdates bool
	}{
		{
			name:           "watched stock of an uncapped rostered market",
			symbol:         "2330",
			market:         vo.MarketTaiwanStock,
			catalog:        uncappedCatalog,
			hasLiveUpdates: true,
		},
		{
			name:           "unwatched stock of an uncapped rostered market",
			symbol:         "2454",
			market:         vo.MarketTaiwanStock,
			catalog:        uncappedCatalog,
			hasLiveUpdates: false,
		},
		{
			name:           "anything at all of a market followed by whoever looks",
			symbol:         "BTCUSDT",
			market:         vo.MarketCrypto,
			catalog:        marketCatalog(),
			hasLiveUpdates: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.hasLiveUpdates, rosterDomain.HasLiveUpdates(
				testCase.symbol, testCase.catalog.MarketOf(string(testCase.market))))
		})
	}
}

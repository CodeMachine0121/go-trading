package vo_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

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

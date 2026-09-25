package vo_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

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

func TestTheSameSymbolOnTwoMarketsIsTwoChannels(t *testing.T) {
	assert.NotEqual(t,
		vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}).Key,
		vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{"2330"}).Key)
}

func TestBuildingAChannelLeavesTheCallersListAlone(t *testing.T) {
	symbols := []string{"2454", "2330"}

	vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, symbols)

	assert.Equal(t, []string{"2454", "2330"}, symbols)
}

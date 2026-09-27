package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradeWinTallyDomain is the one place a closed trade of either journal counts as a win: net profit above zero, as a replay's summary counts it.
type TradeWinTallyDomain struct {
	results []vo.TradeResultVo
}

func NewTradeWinTallyDomain(results []vo.TradeResultVo) TradeWinTallyDomain {
	return TradeWinTallyDomain{results: results}
}

func (tallyDomain TradeWinTallyDomain) IsWin(result vo.TradeResultVo) bool {
	return result.NetProfit.IsPositive()
}

func (tallyDomain TradeWinTallyDomain) TradeCount() int {
	return len(tallyDomain.results)
}

func (tallyDomain TradeWinTallyDomain) WinCount() int {
	winCount := 0
	for _, result := range tallyDomain.results {
		if tallyDomain.IsWin(result) {
			winCount++
		}
	}

	return winCount
}

// WinRate is nil for no trades, since no trades is not the same as no wins.
func (tallyDomain TradeWinTallyDomain) WinRate() *float64 {
	if len(tallyDomain.results) == 0 {
		return nil
	}

	winRate := float64(tallyDomain.WinCount()) / float64(len(tallyDomain.results))

	return &winRate
}

// AverageRMultiple leaves out trades without a planned stop and is nil when none has one.
func (tallyDomain TradeWinTallyDomain) AverageRMultiple() *float64 {
	return tallyDomain.averageOf(func(result vo.TradeResultVo) *float64 { return result.RMultiple })
}

// RMultipleTradeCount is how many trades the average R stands on.
func (tallyDomain TradeWinTallyDomain) RMultipleTradeCount() int {
	return tallyDomain.countOf(func(result vo.TradeResultVo) *float64 { return result.RMultiple })
}

func (tallyDomain TradeWinTallyDomain) AverageReturnRate() *float64 {
	return tallyDomain.averageOf(func(result vo.TradeResultVo) *float64 { return result.ReturnRate })
}

// EntrySlippageTradeCount counts only trades started from a bot round, the only ones with a reference price to slip from.
func (tallyDomain TradeWinTallyDomain) EntrySlippageTradeCount() int {
	return tallyDomain.countOf(func(result vo.TradeResultVo) *float64 { return result.EntrySlippagePercentage })
}

// AverageEntrySlippagePercentage is nil when no trade started from a bot round.
func (tallyDomain TradeWinTallyDomain) AverageEntrySlippagePercentage() *float64 {
	return tallyDomain.averageOf(func(result vo.TradeResultVo) *float64 { return result.EntrySlippagePercentage })
}

func (tallyDomain TradeWinTallyDomain) OfDirection(direction vo.PositionDirectionVo) TradeWinTallyDomain {
	return tallyDomain.where(func(result vo.TradeResultVo) bool {
		return result.Direction == direction
	})
}

func (tallyDomain TradeWinTallyDomain) FollowingATradingStrategy(following bool) TradeWinTallyDomain {
	return tallyDomain.where(func(result vo.TradeResultVo) bool {
		return result.FollowsTradingStrategy == following
	})
}

func (tallyDomain TradeWinTallyDomain) where(keeps func(result vo.TradeResultVo) bool) TradeWinTallyDomain {
	return TradeWinTallyDomain{results: slices.DeleteFunc(slices.Clone(tallyDomain.results),
		func(result vo.TradeResultVo) bool { return !keeps(result) })}
}

func (tallyDomain TradeWinTallyDomain) countOf(figureOf func(result vo.TradeResultVo) *float64) int {
	counted := 0
	for _, result := range tallyDomain.results {
		if figureOf(result) != nil {
			counted++
		}
	}

	return counted
}

func (tallyDomain TradeWinTallyDomain) averageOf(figureOf func(result vo.TradeResultVo) *float64) *float64 {
	total := 0.0
	counted := 0
	for _, result := range tallyDomain.results {
		if figure := figureOf(result); figure != nil {
			total += *figure
			counted++
		}
	}

	if counted == 0 {
		return nil
	}

	average := total / float64(counted)

	return &average
}

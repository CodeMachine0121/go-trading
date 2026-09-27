package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SpotTradeResultsDomain hands closed spot trades to the shared win tally.
type SpotTradeResultsDomain struct {
	trades []dto.SpotTradeRecordDto
}

func NewSpotTradeResultsDomain(trades []dto.SpotTradeRecordDto) SpotTradeResultsDomain {
	return SpotTradeResultsDomain{trades: trades}
}

func (resultsDomain SpotTradeResultsDomain) Tally() TradeWinTallyDomain {
	results := make([]vo.TradeResultVo, 0, len(resultsDomain.trades))
	for _, trade := range resultsDomain.trades {
		results = append(results, vo.TradeResultVo{
			NetProfit:               trade.Outcome.NetProfit,
			RMultiple:               trade.Outcome.RMultiple,
			ReturnRate:              trade.Outcome.ReturnRate,
			EntrySlippagePercentage: trade.Outcome.EntrySlippagePercentage,
			Direction:               vo.PositionDirectionLong,
			FollowsTradingStrategy:  trade.TradingStrategyID != nil,
		})
	}

	return NewTradeWinTallyDomain(results)
}

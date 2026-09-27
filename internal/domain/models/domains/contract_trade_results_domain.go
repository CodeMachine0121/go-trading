package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractTradeResultsDomain hands closed contract trades to the shared win tally.
type ContractTradeResultsDomain struct {
	trades []dto.ContractTradeRecordDto
}

func NewContractTradeResultsDomain(trades []dto.ContractTradeRecordDto) ContractTradeResultsDomain {
	return ContractTradeResultsDomain{trades: trades}
}

func (resultsDomain ContractTradeResultsDomain) Tally() TradeWinTallyDomain {
	results := make([]vo.TradeResultVo, 0, len(resultsDomain.trades))
	for _, trade := range resultsDomain.trades {
		results = append(results, vo.TradeResultVo{
			NetProfit:               trade.Outcome.NetProfit,
			RMultiple:               trade.Outcome.RMultiple,
			EntrySlippagePercentage: trade.Outcome.EntrySlippagePercentage,
			Direction:               vo.PositionDirectionVo(trade.Direction),
			FollowsTradingStrategy:  trade.TradingStrategyID != nil,
		})
	}

	return NewTradeWinTallyDomain(results)
}

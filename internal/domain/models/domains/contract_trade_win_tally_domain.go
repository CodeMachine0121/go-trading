package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractTradeWinTallyDomain is the one place a closed trade counts as a win: net profit above zero, as a replay's summary counts it.
type ContractTradeWinTallyDomain struct {
	trades []dto.ContractTradeRecordDto
}

func NewContractTradeWinTallyDomain(trades []dto.ContractTradeRecordDto) ContractTradeWinTallyDomain {
	return ContractTradeWinTallyDomain{trades: trades}
}

func (tallyDomain ContractTradeWinTallyDomain) IsWin(trade dto.ContractTradeRecordDto) bool {
	return trade.Outcome.NetProfit.IsPositive()
}

func (tallyDomain ContractTradeWinTallyDomain) TradeCount() int {
	return len(tallyDomain.trades)
}

func (tallyDomain ContractTradeWinTallyDomain) WinCount() int {
	winCount := 0
	for _, trade := range tallyDomain.trades {
		if tallyDomain.IsWin(trade) {
			winCount++
		}
	}

	return winCount
}

// WinRate is nil for no trades, since no trades is not the same as no wins.
func (tallyDomain ContractTradeWinTallyDomain) WinRate() *float64 {
	if len(tallyDomain.trades) == 0 {
		return nil
	}

	winRate := float64(tallyDomain.WinCount()) / float64(len(tallyDomain.trades))

	return &winRate
}

// AverageRMultiple leaves out trades without a planned stop and is nil when none has one.
func (tallyDomain ContractTradeWinTallyDomain) AverageRMultiple() *float64 {
	total := 0.0
	counted := 0
	for _, trade := range tallyDomain.trades {
		if trade.Outcome.RMultiple != nil {
			total += *trade.Outcome.RMultiple
			counted++
		}
	}

	if counted == 0 {
		return nil
	}

	average := total / float64(counted)

	return &average
}

func (tallyDomain ContractTradeWinTallyDomain) OfDirection(direction vo.PositionDirectionVo) ContractTradeWinTallyDomain {
	return tallyDomain.where(func(trade dto.ContractTradeRecordDto) bool {
		return trade.Direction == string(direction)
	})
}

func (tallyDomain ContractTradeWinTallyDomain) FollowingATradingStrategy(following bool) ContractTradeWinTallyDomain {
	return tallyDomain.where(func(trade dto.ContractTradeRecordDto) bool {
		return (trade.TradingStrategyID != nil) == following
	})
}

func (tallyDomain ContractTradeWinTallyDomain) where(
	keeps func(trade dto.ContractTradeRecordDto) bool,
) ContractTradeWinTallyDomain {
	return ContractTradeWinTallyDomain{trades: slices.DeleteFunc(slices.Clone(tallyDomain.trades),
		func(trade dto.ContractTradeRecordDto) bool { return !keeps(trade) })}
}

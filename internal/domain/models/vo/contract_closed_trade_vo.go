package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractClosedTradeVo is one finished round trip of a contract replay, settled once
// at its exit and never changed. Profit is net of both charges and of the funding.
type ContractClosedTradeVo struct {
	Direction  PositionDirectionVo
	EntryTime  time.Time
	EntryPrice decimal.Decimal
	ExitTime   time.Time
	ExitPrice  decimal.Decimal
	Leverage   decimal.Decimal
	Quantity   decimal.Decimal
	Margin     decimal.Decimal
	EntryCost  decimal.Decimal
	ExitCost   decimal.Decimal
	FundingFee decimal.Decimal
	Profit     decimal.Decimal
	ExitReason TradeExitReasonVo
}

// IsWin is whether the round trip made money once everything it paid is counted.
func (contractClosedTradeVo ContractClosedTradeVo) IsWin() bool {
	return contractClosedTradeVo.Profit.IsPositive()
}

func (contractClosedTradeVo ContractClosedTradeVo) ToDto() dto.ContractClosedTradeDto {
	return dto.ContractClosedTradeDto{
		Direction:  string(contractClosedTradeVo.Direction),
		EntryTime:  contractClosedTradeVo.EntryTime.UTC(),
		EntryPrice: contractClosedTradeVo.EntryPrice,
		ExitTime:   contractClosedTradeVo.ExitTime.UTC(),
		ExitPrice:  contractClosedTradeVo.ExitPrice,
		Leverage:   contractClosedTradeVo.Leverage,
		Quantity:   contractClosedTradeVo.Quantity,
		Margin:     contractClosedTradeVo.Margin,
		EntryCost:  contractClosedTradeVo.EntryCost,
		ExitCost:   contractClosedTradeVo.ExitCost,
		FundingFee: contractClosedTradeVo.FundingFee,
		Profit:     contractClosedTradeVo.Profit,
		ExitReason: string(contractClosedTradeVo.ExitReason),
	}
}

// ToOutcomeVo is this round trip as the trade statistics read it. Funding is not a
// charge for trading — it has its own figure on the report card — so what the trade
// made before its charges puts back the funding it paid as well as both charges.
func (contractClosedTradeVo ContractClosedTradeVo) ToOutcomeVo() TradeOutcomeVo {
	transactionCost := contractClosedTradeVo.EntryCost.Add(contractClosedTradeVo.ExitCost)

	return TradeOutcomeVo{
		NetProfit: contractClosedTradeVo.Profit,
		GrossProfit: contractClosedTradeVo.Profit.Add(transactionCost).
			Add(contractClosedTradeVo.FundingFee),
		TransactionCost: transactionCost,
		EntryTime:       contractClosedTradeVo.EntryTime,
		ExitTime:        contractClosedTradeVo.ExitTime,
	}
}

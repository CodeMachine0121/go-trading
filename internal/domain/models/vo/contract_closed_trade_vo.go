package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractClosedTradeVo is one finished contract round trip; Profit is net of both charges and funding.
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

// ToOutcomeVo adds back both charges and the funding, since funding is reported separately from trading charges.
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

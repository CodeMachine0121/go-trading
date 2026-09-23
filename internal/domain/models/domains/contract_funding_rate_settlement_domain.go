package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementDomain holds one funding rate settlement and guarantees
// its rules. An instance only exists when every rule passed.
//
// **The rate itself answers to no rule.** Positive means the long side pays the short
// side, negative the other way round, and zero that nobody paid — all three are the
// market saying something, and none of them is a figure this system gets to refuse.
type ContractFundingRateSettlementDomain struct {
	symbol         string
	settlementTime time.Time
	fundingRate    decimal.Decimal
	markPrice      decimal.NullDecimal
}

// NewContractFundingRateSettlementDomain checks one reported settlement, judging "in
// the future" against currentTime.
func NewContractFundingRateSettlementDomain(
	settlementVo vo.ContractFundingRateSettlementVo, currentTime time.Time,
) (ContractFundingRateSettlementDomain, error) {
	contractSymbol, symbolError := NewTradingSymbolDomain(settlementVo.Symbol)
	if symbolError != nil {
		return ContractFundingRateSettlementDomain{}, fmt.Errorf(
			"%w: %w", ErrContractFundingRateSettlementValidation, symbolError)
	}

	if settlementVo.SettlementTime.After(currentTime) {
		return ContractFundingRateSettlementDomain{}, fmt.Errorf(
			"%w: 結算時間不得指向未來", ErrContractFundingRateSettlementValidation)
	}

	// Absent is lawful — the venue's earliest settlements recorded none. A mark price
	// that is there, though, is a price, and a price of zero or less is not one.
	if settlementVo.MarkPrice.Valid && !settlementVo.MarkPrice.Decimal.IsPositive() {
		return ContractFundingRateSettlementDomain{}, fmt.Errorf(
			"%w: 結算當下的標記價格必須大於零", ErrContractFundingRateSettlementValidation)
	}

	return ContractFundingRateSettlementDomain{
		symbol:         contractSymbol.Value(),
		settlementTime: settlementVo.SettlementTime.UTC(),
		fundingRate:    settlementVo.FundingRate,
		markPrice:      settlementVo.MarkPrice,
	}, nil
}

// ToEntity converts this settlement into the record shape that is stored.
func (settlementDomain ContractFundingRateSettlementDomain) ToEntity() entities.ContractFundingRateSettlement {
	return entities.ContractFundingRateSettlement{
		Symbol:         settlementDomain.symbol,
		SettlementTime: settlementDomain.settlementTime,
		FundingRate:    settlementDomain.fundingRate,
		MarkPrice:      settlementDomain.markPrice,
	}
}

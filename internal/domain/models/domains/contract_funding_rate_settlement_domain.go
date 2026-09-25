package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementDomain is a validated settlement; any funding rate sign is accepted (positive means longs pay shorts).
type ContractFundingRateSettlementDomain struct {
	symbol         string
	settlementTime time.Time
	fundingRate    decimal.Decimal
	markPrice      decimal.NullDecimal
}

// NewContractFundingRateSettlementDomain refuses settlements after currentTime.
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

	// An absent mark price is allowed (early settlements lack one), but a present one must be positive.
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

func (settlementDomain ContractFundingRateSettlementDomain) ToEntity() entities.ContractFundingRateSettlement {
	return entities.ContractFundingRateSettlement{
		Symbol:         settlementDomain.symbol,
		SettlementTime: settlementDomain.settlementTime,
		FundingRate:    settlementDomain.fundingRate,
		MarkPrice:      settlementDomain.markPrice,
	}
}

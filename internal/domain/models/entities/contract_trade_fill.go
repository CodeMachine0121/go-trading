package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

type ContractTradeFill struct {
	ID                    uint            `gorm:"primaryKey"`
	ContractTradeRecordID uint            `gorm:"not null;index"`
	Kind                  string          `gorm:"size:8;not null"`
	FilledAt              time.Time       `gorm:"type:timestamptz;not null"`
	Price                 decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Quantity              decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Liquidity             string          `gorm:"size:8;not null"`
	Fee                   decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	FeeRateMissing        bool            `gorm:"not null;default:false"`
}

func (contractTradeFill ContractTradeFill) TableName() string {
	return "ContractTradeFills"
}

func (contractTradeFill ContractTradeFill) ToDto() dto.ContractTradeFillDto {
	return dto.ContractTradeFillDto{
		ID:             contractTradeFill.ID,
		Kind:           contractTradeFill.Kind,
		FilledAt:       contractTradeFill.FilledAt.UTC(),
		Price:          contractTradeFill.Price,
		Quantity:       contractTradeFill.Quantity,
		Liquidity:      contractTradeFill.Liquidity,
		Fee:            contractTradeFill.Fee,
		FeeRateMissing: contractTradeFill.FeeRateMissing,
	}
}

func (contractTradeFill ContractTradeFill) ToTradeLedgerFillVo() vo.TradeLedgerFillVo {
	return vo.TradeLedgerFillVo{
		ID:       contractTradeFill.ID,
		Kind:     vo.ContractTradeFillKindVo(contractTradeFill.Kind),
		FilledAt: contractTradeFill.FilledAt,
		Price:    contractTradeFill.Price,
		Quantity: contractTradeFill.Quantity,
		Fee:      contractTradeFill.Fee,
	}
}

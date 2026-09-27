package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

type SpotTradeFill struct {
	ID                uint            `gorm:"primaryKey"`
	SpotTradeRecordID uint            `gorm:"not null;index"`
	Kind              string          `gorm:"size:8;not null"`
	FilledAt          time.Time       `gorm:"type:timestamptz;not null"`
	Price             decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Quantity          decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Fee               decimal.Decimal `gorm:"type:numeric(38,18);not null"`
}

func (spotTradeFill SpotTradeFill) TableName() string {
	return "SpotTradeFills"
}

func (spotTradeFill SpotTradeFill) ToDto() dto.SpotTradeFillDto {
	return dto.SpotTradeFillDto{
		ID:       spotTradeFill.ID,
		Kind:     spotTradeFill.Kind,
		FilledAt: spotTradeFill.FilledAt.UTC(),
		Price:    spotTradeFill.Price,
		Quantity: spotTradeFill.Quantity,
		Fee:      spotTradeFill.Fee,
	}
}

// ToTradeLedgerFillVo reads a buy as the ledger's entry and a sell as its exit; any other kind is passed through for the ledger to refuse.
func (spotTradeFill SpotTradeFill) ToTradeLedgerFillVo() vo.TradeLedgerFillVo {
	kind := vo.ContractTradeFillKindVo(spotTradeFill.Kind)
	switch vo.SpotTradeFillKindVo(spotTradeFill.Kind) {
	case vo.SpotTradeFillKindBuy:
		kind = vo.ContractTradeFillKindEntry
	case vo.SpotTradeFillKindSell:
		kind = vo.ContractTradeFillKindExit
	}

	return vo.TradeLedgerFillVo{
		ID:       spotTradeFill.ID,
		Kind:     kind,
		FilledAt: spotTradeFill.FilledAt,
		Price:    spotTradeFill.Price,
		Quantity: spotTradeFill.Quantity,
		Fee:      spotTradeFill.Fee,
	}
}

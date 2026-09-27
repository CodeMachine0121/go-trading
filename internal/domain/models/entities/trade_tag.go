package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type TradeTag struct {
	ID      uint   `gorm:"primaryKey"`
	OwnerID uint   `gorm:"not null;uniqueIndex:idx_trade_tags_owner_kind_name,priority:1"`
	Kind    string `gorm:"size:16;not null;uniqueIndex:idx_trade_tags_owner_kind_name,priority:2"`
	Name    string `gorm:"size:64;not null;uniqueIndex:idx_trade_tags_owner_kind_name,priority:3"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
}

func (tradeTag TradeTag) TableName() string {
	return "TradeTags"
}

func (tradeTag TradeTag) ToDto() dto.TradeTagDto {
	return dto.TradeTagDto{ID: tradeTag.ID, Kind: tradeTag.Kind, Name: tradeTag.Name}
}

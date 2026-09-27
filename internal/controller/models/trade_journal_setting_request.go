package models

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// TradeJournalSettingRequest rates are percentages; null clears a rate.
type TradeJournalSettingRequest struct {
	MakerFeeRate decimal.NullDecimal `json:"makerFeeRate"`
	TakerFeeRate decimal.NullDecimal `json:"takerFeeRate"`
}

func (settingRequest TradeJournalSettingRequest) ToWriteDto() dto.TradeJournalSettingWriteDto {
	return dto.TradeJournalSettingWriteDto{
		MakerFeeRate: settingRequest.MakerFeeRate,
		TakerFeeRate: settingRequest.TakerFeeRate,
	}
}

type TradeTagRequest struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func (tagRequest TradeTagRequest) ToWriteDto() dto.TradeTagWriteDto {
	return dto.TradeTagWriteDto{Kind: tagRequest.Kind, Name: tagRequest.Name}
}

type TradeTagRenameRequest struct {
	Name string `json:"name"`
}

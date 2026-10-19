package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// SymbolScreeningProfile keeps the script by id without a cascade, so deleting the script leaves the profile to be refused at the next submission.
type SymbolScreeningProfile struct {
	ID                             uint                                  `gorm:"primaryKey"`
	OwnerID                        uint                                  `gorm:"not null;uniqueIndex:idx_symbol_screening_profiles_owner_id_name,priority:1"`
	Name                           string                                `gorm:"size:128;not null;uniqueIndex:idx_symbol_screening_profiles_owner_id_name,priority:2"`
	StrategyScriptID               uint                                  `gorm:"not null"`
	ParameterValues                []dto.StrategyScriptParameterValueDto `gorm:"serializer:json;type:jsonb;not null"`
	AggregationInterval            string                                `gorm:"size:8;not null"`
	TradingMode                    string                                `gorm:"size:16;not null"`
	Leverage                       decimal.Decimal                       `gorm:"type:numeric(38,18);not null"`
	EntryCostPercentage            decimal.Decimal                       `gorm:"type:numeric(38,18);not null"`
	SlippagePercentage             decimal.Decimal                       `gorm:"type:numeric(38,18);not null"`
	HistoryDays                    int                                   `gorm:"not null"`
	SegmentCount                   int                                   `gorm:"not null"`
	MinimumTradeCount              int                                   `gorm:"not null"`
	MinimumAverageDailyQuoteVolume decimal.Decimal                       `gorm:"type:numeric(38,18);not null"`
	BaselineSymbols                []string                              `gorm:"serializer:json;type:jsonb;not null"`
	CreatedAt                      time.Time                             `gorm:"type:timestamptz;not null"`
	UpdatedAt                      time.Time                             `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
}

func (symbolScreeningProfile SymbolScreeningProfile) TableName() string {
	return "SymbolScreeningProfiles"
}

func (symbolScreeningProfile SymbolScreeningProfile) ToDto() dto.SymbolScreeningProfileDto {
	return dto.SymbolScreeningProfileDto{
		ID:                             symbolScreeningProfile.ID,
		Name:                           symbolScreeningProfile.Name,
		StrategyScriptID:               symbolScreeningProfile.StrategyScriptID,
		ParameterValues:                symbolScreeningProfile.ParameterValues,
		AggregationInterval:            symbolScreeningProfile.AggregationInterval,
		TradingMode:                    symbolScreeningProfile.TradingMode,
		Leverage:                       symbolScreeningProfile.Leverage,
		EntryCostPercentage:            symbolScreeningProfile.EntryCostPercentage,
		SlippagePercentage:             symbolScreeningProfile.SlippagePercentage,
		HistoryDays:                    symbolScreeningProfile.HistoryDays,
		SegmentCount:                   symbolScreeningProfile.SegmentCount,
		MinimumTradeCount:              symbolScreeningProfile.MinimumTradeCount,
		MinimumAverageDailyQuoteVolume: symbolScreeningProfile.MinimumAverageDailyQuoteVolume,
		BaselineSymbols:                symbolScreeningProfile.BaselineSymbols,
		CreatedAt:                      symbolScreeningProfile.CreatedAt.UTC(),
		UpdatedAt:                      symbolScreeningProfile.UpdatedAt.UTC(),
	}
}

// ToWriteDto restates the stored profile as a rewrite, so it can be judged again by the rules that saved it.
func (symbolScreeningProfile SymbolScreeningProfile) ToWriteDto() dto.SymbolScreeningProfileWriteDto {
	return dto.SymbolScreeningProfileWriteDto{
		Name:                           symbolScreeningProfile.Name,
		StrategyScriptID:               symbolScreeningProfile.StrategyScriptID,
		ParameterValues:                symbolScreeningProfile.ParameterValues,
		AggregationInterval:            symbolScreeningProfile.AggregationInterval,
		TradingMode:                    symbolScreeningProfile.TradingMode,
		Leverage:                       &symbolScreeningProfile.Leverage,
		EntryCostPercentage:            &symbolScreeningProfile.EntryCostPercentage,
		SlippagePercentage:             &symbolScreeningProfile.SlippagePercentage,
		HistoryDays:                    &symbolScreeningProfile.HistoryDays,
		SegmentCount:                   &symbolScreeningProfile.SegmentCount,
		MinimumTradeCount:              &symbolScreeningProfile.MinimumTradeCount,
		MinimumAverageDailyQuoteVolume: &symbolScreeningProfile.MinimumAverageDailyQuoteVolume,
		BaselineSymbols:                symbolScreeningProfile.BaselineSymbols,
	}
}

package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// SpotTradeRecord is one spot trade the person bought and sold by hand; it lives apart from contract trades even on the same symbol.
type SpotTradeRecord struct {
	ID      uint   `gorm:"primaryKey"`
	OwnerID uint   `gorm:"not null;index"`
	Symbol  string `gorm:"size:64;not null"`
	Market  string `gorm:"size:32;not null"`
	Status  string `gorm:"size:16;not null"`

	PlannedStopLossPrice   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	PlannedTakeProfitPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	EntryReason            string              `gorm:"type:text;not null;default:''"`
	Confidence             *int

	// TradingStrategyID has no foreign key so deleting the strategy leaves the trade intact.
	TradingStrategyID *uint

	SourceStrategyBotID            *uint
	SourceStrategyBotName          string `gorm:"size:128;not null;default:''"`
	SourceRunNumber                *int
	SourceReferencePrice           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SourceSuggestedStopLossPrice   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SourceSuggestedTakeProfitPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`

	ReviewWentWell  string `gorm:"type:text;not null;default:''"`
	ReviewWentWrong string `gorm:"type:text;not null;default:''"`
	ReviewNextTime  string `gorm:"type:text;not null;default:''"`
	ExecutionScore  *int
	ReviewedAt      *time.Time `gorm:"type:timestamptz"`

	OpenedAt  time.Time  `gorm:"type:timestamptz;not null"`
	ClosedAt  *time.Time `gorm:"type:timestamptz;index"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time  `gorm:"type:timestamptz;not null"`

	Owner User            `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	Fills []SpotTradeFill `gorm:"foreignKey:SpotTradeRecordID;constraint:OnDelete:CASCADE"`
	Notes []SpotTradeNote `gorm:"foreignKey:SpotTradeRecordID;constraint:OnDelete:CASCADE"`
	Tags  []TradeTag      `gorm:"many2many:spot_trade_record_tags;constraint:OnDelete:CASCADE"`
}

func (spotTradeRecord SpotTradeRecord) TableName() string {
	return "SpotTradeRecords"
}

// ToDto leaves the currency and the figures worked out from fills and market data to the domain.
func (spotTradeRecord SpotTradeRecord) ToDto() dto.SpotTradeRecordDto {
	fillDtos := make([]dto.SpotTradeFillDto, 0, len(spotTradeRecord.Fills))
	for _, fill := range spotTradeRecord.Fills {
		fillDtos = append(fillDtos, fill.ToDto())
	}

	noteDtos := make([]dto.SpotTradeNoteDto, 0, len(spotTradeRecord.Notes))
	for _, note := range spotTradeRecord.Notes {
		noteDtos = append(noteDtos, note.ToDto())
	}

	setupTagDtos := []dto.TradeTagDto{}
	mistakeTagDtos := []dto.TradeTagDto{}
	for _, tag := range spotTradeRecord.Tags {
		if tag.Kind == string(vo.TradeTagKindMistake) {
			mistakeTagDtos = append(mistakeTagDtos, tag.ToDto())
			continue
		}
		setupTagDtos = append(setupTagDtos, tag.ToDto())
	}

	recordDto := dto.SpotTradeRecordDto{
		ID:     spotTradeRecord.ID,
		Symbol: spotTradeRecord.Symbol,
		Market: spotTradeRecord.Market,
		Status: spotTradeRecord.Status,
		Plan: dto.SpotTradePlanDto{
			PlannedStopLossPrice:   spotTradeRecord.PlannedStopLossPrice,
			PlannedTakeProfitPrice: spotTradeRecord.PlannedTakeProfitPrice,
			EntryReason:            spotTradeRecord.EntryReason,
			Confidence:             spotTradeRecord.Confidence,
			Locked:                 spotTradeRecord.Status != string(vo.SpotTradeStatusOpen),
		},
		Fills:             fillDtos,
		Notes:             noteDtos,
		SetupTags:         setupTagDtos,
		MistakeTags:       mistakeTagDtos,
		TradingStrategyID: spotTradeRecord.TradingStrategyID,
		OpenedAt:          spotTradeRecord.OpenedAt.UTC(),
	}

	if spotTradeRecord.ClosedAt != nil {
		closedAt := spotTradeRecord.ClosedAt.UTC()
		recordDto.ClosedAt = &closedAt
	}

	if spotTradeRecord.SourceStrategyBotID != nil && spotTradeRecord.SourceRunNumber != nil {
		recordDto.Source = &dto.SpotTradeSourceDto{
			StrategyBotID:            *spotTradeRecord.SourceStrategyBotID,
			StrategyBotName:          spotTradeRecord.SourceStrategyBotName,
			RunNumber:                *spotTradeRecord.SourceRunNumber,
			ReferencePrice:           spotTradeRecord.SourceReferencePrice,
			SuggestedStopLossPrice:   spotTradeRecord.SourceSuggestedStopLossPrice,
			SuggestedTakeProfitPrice: spotTradeRecord.SourceSuggestedTakeProfitPrice,
		}
	}

	if spotTradeRecord.ReviewedAt != nil && spotTradeRecord.ExecutionScore != nil {
		recordDto.Review = &dto.SpotTradeReviewDto{
			WentWell:       spotTradeRecord.ReviewWentWell,
			WentWrong:      spotTradeRecord.ReviewWentWrong,
			NextTime:       spotTradeRecord.ReviewNextTime,
			ExecutionScore: *spotTradeRecord.ExecutionScore,
			ReviewedAt:     spotTradeRecord.ReviewedAt.UTC(),
		}
	}

	return recordDto
}

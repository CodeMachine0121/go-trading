package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractTradeRecord is one trade the person placed by hand on the venue; its fills and notes live and die with it.
type ContractTradeRecord struct {
	ID        uint            `gorm:"primaryKey"`
	OwnerID   uint            `gorm:"not null;index"`
	Symbol    string          `gorm:"size:64;not null"`
	Direction string          `gorm:"size:8;not null"`
	Leverage  decimal.Decimal `gorm:"type:numeric(38,18);not null"`
	Status    string          `gorm:"size:16;not null"`

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

	Owner User                `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	Fills []ContractTradeFill `gorm:"foreignKey:ContractTradeRecordID;constraint:OnDelete:CASCADE"`
	Notes []ContractTradeNote `gorm:"foreignKey:ContractTradeRecordID;constraint:OnDelete:CASCADE"`
	Tags  []TradeTag          `gorm:"many2many:contract_trade_record_tags;constraint:OnDelete:CASCADE"`
}

func (contractTradeRecord ContractTradeRecord) TableName() string {
	return "ContractTradeRecords"
}

// ToDto leaves the figures worked out from fills and market data to the domain.
func (contractTradeRecord ContractTradeRecord) ToDto() dto.ContractTradeRecordDto {
	fillDtos := make([]dto.ContractTradeFillDto, 0, len(contractTradeRecord.Fills))
	for _, fill := range contractTradeRecord.Fills {
		fillDtos = append(fillDtos, fill.ToDto())
	}

	noteDtos := make([]dto.ContractTradeNoteDto, 0, len(contractTradeRecord.Notes))
	for _, note := range contractTradeRecord.Notes {
		noteDtos = append(noteDtos, note.ToDto())
	}

	setupTagDtos := []dto.TradeTagDto{}
	mistakeTagDtos := []dto.TradeTagDto{}
	for _, tag := range contractTradeRecord.Tags {
		if tag.Kind == string(vo.TradeTagKindMistake) {
			mistakeTagDtos = append(mistakeTagDtos, tag.ToDto())
			continue
		}
		setupTagDtos = append(setupTagDtos, tag.ToDto())
	}

	recordDto := dto.ContractTradeRecordDto{
		ID:        contractTradeRecord.ID,
		Symbol:    contractTradeRecord.Symbol,
		Direction: contractTradeRecord.Direction,
		Leverage:  contractTradeRecord.Leverage,
		Status:    contractTradeRecord.Status,
		Plan: dto.ContractTradePlanDto{
			PlannedStopLossPrice:   contractTradeRecord.PlannedStopLossPrice,
			PlannedTakeProfitPrice: contractTradeRecord.PlannedTakeProfitPrice,
			EntryReason:            contractTradeRecord.EntryReason,
			Confidence:             contractTradeRecord.Confidence,
			Locked:                 contractTradeRecord.Status != string(vo.ContractTradeStatusOpen),
		},
		Fills:             fillDtos,
		Notes:             noteDtos,
		SetupTags:         setupTagDtos,
		MistakeTags:       mistakeTagDtos,
		TradingStrategyID: contractTradeRecord.TradingStrategyID,
		OpenedAt:          contractTradeRecord.OpenedAt.UTC(),
	}

	if contractTradeRecord.ClosedAt != nil {
		closedAt := contractTradeRecord.ClosedAt.UTC()
		recordDto.ClosedAt = &closedAt
	}

	if contractTradeRecord.SourceStrategyBotID != nil && contractTradeRecord.SourceRunNumber != nil {
		recordDto.Source = &dto.ContractTradeSourceDto{
			StrategyBotID:            *contractTradeRecord.SourceStrategyBotID,
			StrategyBotName:          contractTradeRecord.SourceStrategyBotName,
			RunNumber:                *contractTradeRecord.SourceRunNumber,
			ReferencePrice:           contractTradeRecord.SourceReferencePrice,
			SuggestedStopLossPrice:   contractTradeRecord.SourceSuggestedStopLossPrice,
			SuggestedTakeProfitPrice: contractTradeRecord.SourceSuggestedTakeProfitPrice,
		}
	}

	if contractTradeRecord.ReviewedAt != nil && contractTradeRecord.ExecutionScore != nil {
		recordDto.Review = &dto.ContractTradeReviewDto{
			WentWell:       contractTradeRecord.ReviewWentWell,
			WentWrong:      contractTradeRecord.ReviewWentWrong,
			NextTime:       contractTradeRecord.ReviewNextTime,
			ExecutionScore: *contractTradeRecord.ExecutionScore,
			ReviewedAt:     contractTradeRecord.ReviewedAt.UTC(),
		}
	}

	return recordDto
}

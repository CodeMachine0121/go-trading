package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractAutoOrder is what one contract round asked to be done with real money, queued in the same transaction as the round and carried out step by step by whichever replica takes it.
// Each finished step is written back before the next begins, so a replica taking over resumes rather than starts again.
type ContractAutoOrder struct {
	ID uint `gorm:"primaryKey"`
	// StrategyBotID, Symbol and RoundDueAt identify one watched symbol's round, so a round booked twice still queues one order per symbol.
	StrategyBotID uint      `gorm:"not null;uniqueIndex:idx_contract_auto_orders_bot_symbol_round,priority:1"`
	Symbol        string    `gorm:"size:64;not null;uniqueIndex:idx_contract_auto_orders_bot_symbol_round,priority:2"`
	RoundDueAt    time.Time `gorm:"type:timestamptz;not null;uniqueIndex:idx_contract_auto_orders_bot_symbol_round,priority:3"`
	// RunNumber names the round in the bot's history, so the result can be shown beside it.
	RunNumber   int  `gorm:"not null"`
	OwnerUserID uint `gorm:"not null"`
	// TargetPosition is long, short or flat.
	TargetPosition string `gorm:"size:8;not null"`
	// OpenQuantity is absent when the round had nothing the venue would accept; NoOpenReason then says why.
	OpenQuantity         decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	NoOpenReason         string              `gorm:"type:text;not null;default:''"`
	Leverage             decimal.Decimal     `gorm:"type:numeric(38,18);not null;default:0"`
	StopLossPercentage   decimal.Decimal     `gorm:"type:numeric(38,18);not null;default:0"`
	TakeProfitPercentage decimal.Decimal     `gorm:"type:numeric(38,18);not null;default:0"`
	// ReferencePrice is the round's reference price; the entry order's limit is worked out from it and EntrySlippageTolerancePercentage, as the bot was set when the round was booked in.
	ReferencePrice                   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	EntrySlippageTolerancePercentage decimal.Decimal     `gorm:"type:numeric(38,18);not null;default:0"`
	// EntryFillDeadline is one trigger interval after the round ran; an entry order still working then is cancelled.
	EntryFillDeadline *time.Time `gorm:"type:timestamptz"`

	Status        string    `gorm:"size:16;not null;index:idx_contract_auto_orders_unsettled"`
	AttemptCount  int       `gorm:"not null;default:0"`
	NextAttemptAt time.Time `gorm:"type:timestamptz;not null"`
	// ClaimedBy and ClaimedUntil say which replica is carrying it out; a claim past its time means that replica died and anyone may resume it.
	ClaimedBy    string     `gorm:"size:255;not null;default:''"`
	ClaimedUntil *time.Time `gorm:"type:timestamptz"`
	// ExpiresAt is when an order not yet sent stops being worth sending.
	ExpiresAt time.Time `gorm:"type:timestamptz;not null"`

	// Close* record the step that closed the bot's own position; ClosePositionVanished is a close that found nothing left at the venue.
	CloseDone             bool                `gorm:"not null;default:false"`
	ClosedDirection       string              `gorm:"size:8;not null;default:''"`
	ClosedQuantity        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	CloseAveragePrice     decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	ClosePositionVanished bool                `gorm:"not null;default:false"`
	// Entry* record the limit order that opens the new position while it works; EntryPlacedAt is set once the venue has it.
	EntryPlacedAt         *time.Time          `gorm:"type:timestamptz"`
	EntryLimitPrice       decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	EntryExchangeOrderID  string              `gorm:"size:32;not null;default:''"`
	EntryExecutedQuantity decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	EntryStatus           string              `gorm:"size:16;not null;default:''"`
	// Open* record the step that opened the new position.
	OpenDone         bool                `gorm:"not null;default:false"`
	OpenedQuantity   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	OpenAveragePrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	OpenedAt         *time.Time          `gorm:"type:timestamptz"`
	// StopLossPrice and TakeProfitPrice are worked out from the fill once the open is done.
	StopLossPrice    decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakeProfitPrice  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	StopLossPlaced   bool                `gorm:"not null;default:false"`
	TakeProfitPlaced bool                `gorm:"not null;default:false"`
	ProtectionDone   bool                `gorm:"not null;default:false"`

	// Outcome and Reason are set when the order is settled; ProtectionMissing is a position left without the guard it should have.
	Outcome           string     `gorm:"size:16;not null;default:''"`
	Reason            string     `gorm:"type:text;not null;default:''"`
	ProtectionMissing bool       `gorm:"not null;default:false"`
	CreatedAt         time.Time  `gorm:"type:timestamptz;not null"`
	SettledAt         *time.Time `gorm:"type:timestamptz"`
}

func (contractAutoOrder ContractAutoOrder) TableName() string {
	return "ContractAutoOrders"
}

// ToResultDto shows an unsettled order as pending whatever it has done so far, since its result is not yet final.
func (contractAutoOrder ContractAutoOrder) ToResultDto(action string) dto.ContractAutoOrderResultDto {
	resultDto := dto.ContractAutoOrderResultDto{
		Status:                "pending",
		Action:                action,
		ReferencePrice:        figureOrNothing(contractAutoOrder.ReferencePrice),
		RequestedQuantity:     figureOrNothing(contractAutoOrder.OpenQuantity),
		EntryLimitPrice:       figureOrNothing(contractAutoOrder.EntryLimitPrice),
		EntryExecutedQuantity: figureOrNothing(contractAutoOrder.EntryExecutedQuantity),
		EntryStatus:           contractAutoOrder.EntryStatus,
		ClosedQuantity:        figureOrNothing(contractAutoOrder.ClosedQuantity),
		CloseAveragePrice:     figureOrNothing(contractAutoOrder.CloseAveragePrice),
		OpenedQuantity:        figureOrNothing(contractAutoOrder.OpenedQuantity),
		OpenAveragePrice:      figureOrNothing(contractAutoOrder.OpenAveragePrice),
		StopLossPrice:         figureOrNothing(contractAutoOrder.StopLossPrice),
		TakeProfitPrice:       figureOrNothing(contractAutoOrder.TakeProfitPrice),
		ProtectionMissing:     contractAutoOrder.ProtectionMissing,
		Reason:                contractAutoOrder.Reason,
	}
	if contractAutoOrder.ReferencePrice.Valid {
		resultDto.EntrySlippageTolerancePercentage = &contractAutoOrder.EntrySlippageTolerancePercentage
	}
	if contractAutoOrder.OpenDone {
		resultDto.OpenedDirection = contractAutoOrder.TargetPosition
	}
	if contractAutoOrder.Outcome != "" {
		resultDto.Status = contractAutoOrder.Outcome
	}

	return resultDto
}

package domains

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractTradeRecordDomain is the only way a trade changes, so a closed trade cannot be quietly rewritten.
type ContractTradeRecordDomain struct {
	record entities.ContractTradeRecord
	ledger ContractTradeLedgerDomain
}

func NewContractTradeRecordDomain(record entities.ContractTradeRecord) ContractTradeRecordDomain {
	return ContractTradeRecordDomain{record: record, ledger: NewContractTradeLedgerDomain(record.Fills)}
}

// NewOpeningContractTradeRecordDomain opens a trade from its first entry fill; the symbol arrives already confirmed as a known contract.
func NewOpeningContractTradeRecordDomain(
	ownerID uint, symbol string, writeDto dto.ContractTradeRecordWriteDto,
	firstEntryFill entities.ContractTradeFill, setupTags []entities.TradeTag, now time.Time,
) (ContractTradeRecordDomain, error) {
	direction := vo.PositionDirectionVo(strings.TrimSpace(writeDto.Direction))
	if direction != vo.PositionDirectionLong && direction != vo.PositionDirectionShort {
		return ContractTradeRecordDomain{}, fmt.Errorf(
			"%w: 方向只有做多（long）與做空（short）", ErrContractTradeValidation)
	}

	leverage := writeDto.Leverage
	if leverage.IsZero() {
		leverage = oneWhole
	}
	if leverage.LessThan(oneWhole) {
		return ContractTradeRecordDomain{}, fmt.Errorf(
			"%w: 槓桿倍數不得小於一", ErrContractTradeValidation)
	}

	// The first fill can only be an entry, so a caller that leaves its kind out means exactly that.
	if strings.TrimSpace(firstEntryFill.Kind) == "" {
		firstEntryFill.Kind = string(vo.ContractTradeFillKindEntry)
	}
	if firstEntryFill.Kind != string(vo.ContractTradeFillKindEntry) {
		return ContractTradeRecordDomain{}, fmt.Errorf(
			"%w: 一筆交易至少要有一筆進場成交", ErrContractTradeValidation)
	}

	ledger, fillError := NewContractTradeLedgerDomain(nil).Admit(firstEntryFill, now)
	if fillError != nil {
		return ContractTradeRecordDomain{}, fillError
	}

	recordDomain := ContractTradeRecordDomain{
		record: entities.ContractTradeRecord{
			OwnerID:           ownerID,
			Symbol:            symbol,
			Direction:         string(direction),
			Leverage:          leverage,
			Status:            string(vo.ContractTradeStatusOpen),
			TradingStrategyID: writeDto.TradingStrategyID,
			OpenedAt:          ledger.FirstEntryAt(),
		},
		ledger: ledger,
	}

	if planError := recordDomain.AmendPlan(writeDto.Plan); planError != nil {
		return ContractTradeRecordDomain{}, planError
	}

	if tagError := recordDomain.AssignSetupTags(setupTags); tagError != nil {
		return ContractTradeRecordDomain{}, tagError
	}

	return recordDomain, nil
}

func (recordDomain ContractTradeRecordDomain) Direction() vo.PositionDirectionVo {
	return vo.PositionDirectionVo(recordDomain.record.Direction)
}

func (recordDomain ContractTradeRecordDomain) DirectionInWords() string {
	if recordDomain.Direction() == vo.PositionDirectionShort {
		return "做空"
	}

	return "做多"
}

func (recordDomain ContractTradeRecordDomain) Ledger() ContractTradeLedgerDomain {
	return recordDomain.ledger
}

func (recordDomain ContractTradeRecordDomain) IsOpen() bool {
	return recordDomain.record.Status == string(vo.ContractTradeStatusOpen)
}

// WithSource copies the bot round's suggestion onto the trade, since the bot forgets old rounds.
func (recordDomain *ContractTradeRecordDomain) WithSource(
	strategyBot entities.StrategyBot, runRecord entities.StrategyBotRunRecord,
) {
	strategyBotID := strategyBot.ID
	runNumber := runRecord.RunNumber

	recordDomain.record.SourceStrategyBotID = &strategyBotID
	recordDomain.record.SourceStrategyBotName = strategyBot.Name
	recordDomain.record.SourceRunNumber = &runNumber
	recordDomain.record.SourceReferencePrice = runRecord.ReferencePrice
	recordDomain.record.SourceSuggestedStopLossPrice = runRecord.SuggestedStopLossPrice
	recordDomain.record.SourceSuggestedTakeProfitPrice = runRecord.SuggestedTakeProfitPrice
}

func (recordDomain *ContractTradeRecordDomain) AddFill(fill entities.ContractTradeFill, now time.Time) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 這筆交易已經平倉，不能再加成交", ErrContractTradeLocked)
	}

	ledger, fillError := recordDomain.ledger.Admit(fill, now)
	if fillError != nil {
		return fillError
	}

	return recordDomain.settle(ledger)
}

func (recordDomain *ContractTradeRecordDomain) AmendFill(
	fillID uint, fill entities.ContractTradeFill, now time.Time,
) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後成交已鎖定，可以加附註或刪除整筆重記", ErrContractTradeLocked)
	}

	ledger, fillError := recordDomain.ledger.Amend(fillID, fill, now)
	if fillError != nil {
		return fillError
	}

	return recordDomain.settle(ledger)
}

func (recordDomain *ContractTradeRecordDomain) RemoveFill(fillID uint, now time.Time) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後成交已鎖定，可以加附註或刪除整筆重記", ErrContractTradeLocked)
	}

	ledger, fillError := recordDomain.ledger.Remove(fillID, now)
	if fillError != nil {
		return fillError
	}

	return recordDomain.settle(ledger)
}

// settle closes the trade the moment its position reaches exactly zero, dated by the last fill rather than by when it was typed in.
func (recordDomain *ContractTradeRecordDomain) settle(ledger ContractTradeLedgerDomain) error {
	if planError := recordDomain.planFitsEntry(
		recordDomain.record.PlannedStopLossPrice, recordDomain.record.PlannedTakeProfitPrice,
		ledger.FirstEntryPrice()); planError != nil {
		return planError
	}

	recordDomain.ledger = ledger
	recordDomain.record.OpenedAt = ledger.FirstEntryAt()

	if ledger.IsFlat() {
		closedAt := ledger.LastFillAt()
		recordDomain.record.Status = string(vo.ContractTradeStatusClosed)
		recordDomain.record.ClosedAt = &closedAt
	}

	return nil
}

func (recordDomain *ContractTradeRecordDomain) AmendPlan(planDto dto.ContractTradePlanWriteDto) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後計畫已鎖定，可以加附註", ErrContractTradeLocked)
	}

	if planDto.Confidence != nil && (*planDto.Confidence < 1 || *planDto.Confidence > 5) {
		return fmt.Errorf("%w: 信心只能是 1 到 5", ErrContractTradeValidation)
	}

	if planError := recordDomain.planFitsEntry(
		planDto.PlannedStopLossPrice, planDto.PlannedTakeProfitPrice,
		recordDomain.ledger.FirstEntryPrice()); planError != nil {
		return planError
	}

	recordDomain.record.PlannedStopLossPrice = planDto.PlannedStopLossPrice
	recordDomain.record.PlannedTakeProfitPrice = planDto.PlannedTakeProfitPrice
	recordDomain.record.EntryReason = strings.TrimSpace(planDto.EntryReason)
	recordDomain.record.Confidence = planDto.Confidence

	return nil
}

// planFitsEntry keeps the stop on the losing side and the target on the winning side of the first entry.
func (recordDomain ContractTradeRecordDomain) planFitsEntry(
	plannedStopLossPrice decimal.NullDecimal, plannedTakeProfitPrice decimal.NullDecimal,
	firstEntryPrice decimal.Decimal,
) error {
	isShort := recordDomain.Direction() == vo.PositionDirectionShort

	if plannedStopLossPrice.Valid {
		if plannedStopLossPrice.Decimal.IsNegative() {
			return fmt.Errorf("%w: 計畫止損不得為負", ErrContractTradeValidation)
		}
		if !isShort && !plannedStopLossPrice.Decimal.LessThan(firstEntryPrice) {
			return fmt.Errorf("%w: 做多的止損必須低於進場價", ErrContractTradeValidation)
		}
		if isShort && !plannedStopLossPrice.Decimal.GreaterThan(firstEntryPrice) {
			return fmt.Errorf("%w: 做空的止損必須高於進場價", ErrContractTradeValidation)
		}
	}

	if plannedTakeProfitPrice.Valid {
		if plannedTakeProfitPrice.Decimal.IsNegative() {
			return fmt.Errorf("%w: 計畫止盈不得為負", ErrContractTradeValidation)
		}
		if !isShort && !plannedTakeProfitPrice.Decimal.GreaterThan(firstEntryPrice) {
			return fmt.Errorf("%w: 做多的止盈必須高於進場價", ErrContractTradeValidation)
		}
		if isShort && !plannedTakeProfitPrice.Decimal.LessThan(firstEntryPrice) {
			return fmt.Errorf("%w: 做空的止盈必須低於進場價", ErrContractTradeValidation)
		}
	}

	return nil
}

func (recordDomain *ContractTradeRecordDomain) AddNote(content string, now time.Time) error {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return fmt.Errorf("%w: 附註不得為空白", ErrContractTradeValidation)
	}

	recordDomain.record.Notes = append(recordDomain.record.Notes, entities.ContractTradeNote{
		ContractTradeRecordID: recordDomain.record.ID,
		Content:               trimmedContent,
		CreatedAt:             now.UTC(),
	})

	return nil
}

func (recordDomain *ContractTradeRecordDomain) WriteReview(
	reviewDto dto.ContractTradeReviewWriteDto, mistakeTags []entities.TradeTag, now time.Time,
) error {
	if recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後才能檢討", ErrContractTradeValidation)
	}

	if reviewDto.ExecutionScore < 1 || reviewDto.ExecutionScore > 5 {
		return fmt.Errorf("%w: 執行評分只能是 1 到 5", ErrContractTradeValidation)
	}

	if tagError := recordDomain.replaceTagsOfKind(vo.TradeTagKindMistake, mistakeTags); tagError != nil {
		return tagError
	}

	executionScore := reviewDto.ExecutionScore
	reviewedAt := now.UTC()

	recordDomain.record.ReviewWentWell = strings.TrimSpace(reviewDto.WentWell)
	recordDomain.record.ReviewWentWrong = strings.TrimSpace(reviewDto.WentWrong)
	recordDomain.record.ReviewNextTime = strings.TrimSpace(reviewDto.NextTime)
	recordDomain.record.ExecutionScore = &executionScore
	recordDomain.record.ReviewedAt = &reviewedAt
	recordDomain.record.Status = string(vo.ContractTradeStatusReviewed)

	return nil
}

func (recordDomain *ContractTradeRecordDomain) AssignSetupTags(setupTags []entities.TradeTag) error {
	return recordDomain.replaceTagsOfKind(vo.TradeTagKindSetup, setupTags)
}

// replaceTagsOfKind leaves the other kind of tag alone, since setup tags and mistake tags are written at different moments.
func (recordDomain *ContractTradeRecordDomain) replaceTagsOfKind(
	kind vo.TradeTagKindVo, tags []entities.TradeTag,
) error {
	for _, tag := range tags {
		if tag.Kind != string(kind) {
			return fmt.Errorf("%w: 標籤「%s」不能用在這裡", ErrContractTradeValidation, tag.Name)
		}
	}

	keptTags := slices.DeleteFunc(slices.Clone(recordDomain.record.Tags), func(tag entities.TradeTag) bool {
		return tag.Kind == string(kind)
	})
	recordDomain.record.Tags = append(keptTags, tags...)

	return nil
}

func (recordDomain ContractTradeRecordDomain) ToEntity() entities.ContractTradeRecord {
	entity := recordDomain.record
	entity.Fills = recordDomain.ledger.Fills()

	return entity
}

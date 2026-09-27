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

// spotTradeLedgerWording names a spot trade's entries and exits as buys and sells.
var spotTradeLedgerWording = vo.TradeLedgerWordingVo{
	Entry:           "買進",
	Exit:            "賣出",
	Holding:         "持有",
	Price:           "價格",
	ValidationError: ErrSpotTradeValidation,
}

// SpotTradeRecordDomain is the only way a spot trade changes: bought first, sold later, never leveraged.
type SpotTradeRecordDomain struct {
	record entities.SpotTradeRecord
}

func NewSpotTradeRecordDomain(record entities.SpotTradeRecord) SpotTradeRecordDomain {
	return SpotTradeRecordDomain{record: record}
}

// NewOpeningSpotTradeRecordDomain opens a trade from its first buy; the symbol and its market arrive already confirmed.
func NewOpeningSpotTradeRecordDomain(
	ownerID uint, symbol string, market string, writeDto dto.SpotTradeRecordWriteDto,
	firstBuyFill entities.SpotTradeFill, setupTags []entities.TradeTag, now time.Time,
) (SpotTradeRecordDomain, error) {
	if writeDto.Leverage.Valid || strings.TrimSpace(writeDto.Direction) != "" {
		return SpotTradeRecordDomain{}, fmt.Errorf("%w: 現貨只有先買後賣，沒有槓桿", ErrSpotTradeValidation)
	}

	// The first fill can only be a buy, so a caller that leaves its kind out means exactly that.
	if strings.TrimSpace(firstBuyFill.Kind) == "" {
		firstBuyFill.Kind = string(vo.SpotTradeFillKindBuy)
	}
	if firstBuyFill.Kind != string(vo.SpotTradeFillKindBuy) {
		return SpotTradeRecordDomain{}, fmt.Errorf("%w: 現貨只有先買後賣，第一筆必須是買進", ErrSpotTradeValidation)
	}

	recordDomain := SpotTradeRecordDomain{
		record: entities.SpotTradeRecord{
			OwnerID:           ownerID,
			Symbol:            symbol,
			Market:            string(NewSpotTradeMarketDomain(market).Market()),
			Status:            string(vo.SpotTradeStatusOpen),
			TradingStrategyID: writeDto.TradingStrategyID,
		},
	}
	if fillError := recordDomain.settle([]entities.SpotTradeFill{firstBuyFill}, now); fillError != nil {
		return SpotTradeRecordDomain{}, fillError
	}

	if planError := recordDomain.AmendPlan(writeDto.Plan); planError != nil {
		return SpotTradeRecordDomain{}, planError
	}

	if tagError := recordDomain.AssignSetupTags(setupTags); tagError != nil {
		return SpotTradeRecordDomain{}, tagError
	}

	return recordDomain, nil
}

func (recordDomain SpotTradeRecordDomain) Ledger() TradeLedgerDomain {
	return recordDomain.ledgerOf(recordDomain.record.Fills)
}

func (recordDomain SpotTradeRecordDomain) MarketDomain() SpotTradeMarketDomain {
	return NewSpotTradeMarketDomain(recordDomain.record.Market)
}

func (recordDomain SpotTradeRecordDomain) IsOpen() bool {
	return recordDomain.record.Status == string(vo.SpotTradeStatusOpen)
}

// WithSource copies the bot round's suggestion onto the trade, since the bot forgets old rounds.
// WithSource keeps a round only when it was about this very symbol, so its reference price never skews another trade's slippage.
func (recordDomain *SpotTradeRecordDomain) WithSource(round dto.JournalLinkRoundDto) {
	if round.Symbol != recordDomain.record.Symbol {
		return
	}

	strategyBotID := round.StrategyBotID
	runNumber := round.RunNumber

	recordDomain.record.SourceStrategyBotID = &strategyBotID
	recordDomain.record.SourceStrategyBotName = round.StrategyBotName
	recordDomain.record.SourceRunNumber = &runNumber
	recordDomain.record.SourceReferencePrice = round.ReferencePrice
	recordDomain.record.SourceSuggestedStopLossPrice = round.SuggestedStopLossPrice
	recordDomain.record.SourceSuggestedTakeProfitPrice = round.SuggestedTakeProfitPrice
}

func (recordDomain *SpotTradeRecordDomain) AddFill(fill entities.SpotTradeFill, now time.Time) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 這筆交易已經平倉，不能再加買進或賣出", ErrSpotTradeLocked)
	}

	return recordDomain.settle(append(slices.Clone(recordDomain.record.Fills), fill), now)
}

func (recordDomain *SpotTradeRecordDomain) AmendFill(fillID uint, fill entities.SpotTradeFill, now time.Time) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後買賣已鎖定，可以加附註或刪除整筆重記", ErrSpotTradeLocked)
	}

	index, findError := recordDomain.indexOfFill(fillID)
	if findError != nil {
		return findError
	}

	amendedFills := slices.Clone(recordDomain.record.Fills)
	fill.ID = fillID
	fill.SpotTradeRecordID = recordDomain.record.ID
	amendedFills[index] = fill

	return recordDomain.settle(amendedFills, now)
}

func (recordDomain *SpotTradeRecordDomain) RemoveFill(fillID uint, now time.Time) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後買賣已鎖定，可以加附註或刪除整筆重記", ErrSpotTradeLocked)
	}

	index, findError := recordDomain.indexOfFill(fillID)
	if findError != nil {
		return findError
	}

	return recordDomain.settle(slices.Delete(slices.Clone(recordDomain.record.Fills), index, index+1), now)
}

func (recordDomain SpotTradeRecordDomain) indexOfFill(fillID uint) (int, error) {
	index := slices.IndexFunc(recordDomain.record.Fills, func(fill entities.SpotTradeFill) bool {
		return fill.ID == fillID
	})
	if index < 0 {
		return -1, fmt.Errorf("%w: 這筆交易沒有識別碼為 %d 的買賣", ErrSpotTradeValidation, fillID)
	}

	return index, nil
}

// settle takes the fills only when they still make one trade, and closes it the moment nothing is held, dated by the last sell.
func (recordDomain *SpotTradeRecordDomain) settle(fills []entities.SpotTradeFill, now time.Time) error {
	marketDomain := recordDomain.MarketDomain()
	for _, fill := range fills {
		if quantityError := marketDomain.RequireQuantity(fill.Quantity); quantityError != nil {
			return quantityError
		}
	}

	ledger := recordDomain.ledgerOf(fills)
	if ledgerError := ledger.Validate(now); ledgerError != nil {
		return ledgerError
	}

	if planError := recordDomain.planFitsFirstBuy(
		recordDomain.record.PlannedStopLossPrice, recordDomain.record.PlannedTakeProfitPrice,
		ledger.FirstEntryPrice()); planError != nil {
		return planError
	}

	recordDomain.record.Fills = fills
	recordDomain.record.OpenedAt = ledger.FirstEntryAt()

	if ledger.IsFlat() {
		closedAt := ledger.LastFillAt()
		recordDomain.record.Status = string(vo.SpotTradeStatusClosed)
		recordDomain.record.ClosedAt = &closedAt
	}

	return nil
}

func (recordDomain *SpotTradeRecordDomain) AmendPlan(planDto dto.SpotTradePlanWriteDto) error {
	if !recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後計畫已鎖定，可以加附註", ErrSpotTradeLocked)
	}

	if planDto.Confidence != nil && (*planDto.Confidence < 1 || *planDto.Confidence > 5) {
		return fmt.Errorf("%w: 信心只能是 1 到 5", ErrSpotTradeValidation)
	}

	if planError := recordDomain.planFitsFirstBuy(
		planDto.PlannedStopLossPrice, planDto.PlannedTakeProfitPrice,
		recordDomain.Ledger().FirstEntryPrice()); planError != nil {
		return planError
	}

	recordDomain.record.PlannedStopLossPrice = planDto.PlannedStopLossPrice
	recordDomain.record.PlannedTakeProfitPrice = planDto.PlannedTakeProfitPrice
	recordDomain.record.EntryReason = strings.TrimSpace(planDto.EntryReason)
	recordDomain.record.Confidence = planDto.Confidence

	return nil
}

// planFitsFirstBuy keeps the stop below and the target above the first buy.
func (recordDomain SpotTradeRecordDomain) planFitsFirstBuy(
	plannedStopLossPrice decimal.NullDecimal, plannedTakeProfitPrice decimal.NullDecimal, firstBuyPrice decimal.Decimal,
) error {
	if plannedStopLossPrice.Valid {
		if plannedStopLossPrice.Decimal.IsNegative() {
			return fmt.Errorf("%w: 計畫止損不得為負", ErrSpotTradeValidation)
		}
		if !plannedStopLossPrice.Decimal.LessThan(firstBuyPrice) {
			return fmt.Errorf("%w: 止損必須低於買進價", ErrSpotTradeValidation)
		}
	}

	if plannedTakeProfitPrice.Valid && !plannedTakeProfitPrice.Decimal.GreaterThan(firstBuyPrice) {
		return fmt.Errorf("%w: 止盈必須高於買進價", ErrSpotTradeValidation)
	}

	return nil
}

func (recordDomain *SpotTradeRecordDomain) AddNote(content string, now time.Time) error {
	trimmedContent := strings.TrimSpace(content)
	if trimmedContent == "" {
		return fmt.Errorf("%w: 附註不得為空白", ErrSpotTradeValidation)
	}

	recordDomain.record.Notes = append(recordDomain.record.Notes, entities.SpotTradeNote{
		SpotTradeRecordID: recordDomain.record.ID,
		Content:           trimmedContent,
		CreatedAt:         now.UTC(),
	})

	return nil
}

func (recordDomain *SpotTradeRecordDomain) WriteReview(
	reviewDto dto.SpotTradeReviewWriteDto, mistakeTags []entities.TradeTag, now time.Time,
) error {
	if recordDomain.IsOpen() {
		return fmt.Errorf("%w: 平倉後才能檢討", ErrSpotTradeValidation)
	}

	if reviewDto.ExecutionScore < 1 || reviewDto.ExecutionScore > 5 {
		return fmt.Errorf("%w: 執行評分只能是 1 到 5", ErrSpotTradeValidation)
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
	recordDomain.record.Status = string(vo.SpotTradeStatusReviewed)

	return nil
}

func (recordDomain *SpotTradeRecordDomain) AssignSetupTags(setupTags []entities.TradeTag) error {
	return recordDomain.replaceTagsOfKind(vo.TradeTagKindSetup, setupTags)
}

// replaceTagsOfKind leaves the other kind of tag alone, since setup tags and mistake tags are written at different moments.
func (recordDomain *SpotTradeRecordDomain) replaceTagsOfKind(kind vo.TradeTagKindVo, tags []entities.TradeTag) error {
	for _, tag := range tags {
		if tag.Kind != string(kind) {
			return fmt.Errorf("%w: 標籤「%s」不能用在這裡", ErrSpotTradeValidation, tag.Name)
		}
	}

	keptTags := slices.DeleteFunc(slices.Clone(recordDomain.record.Tags), func(tag entities.TradeTag) bool {
		return tag.Kind == string(kind)
	})
	recordDomain.record.Tags = append(keptTags, tags...)

	return nil
}

func (recordDomain SpotTradeRecordDomain) ToEntity() entities.SpotTradeRecord {
	return recordDomain.record
}

func (recordDomain SpotTradeRecordDomain) ledgerOf(fills []entities.SpotTradeFill) TradeLedgerDomain {
	ledgerFills := make([]vo.TradeLedgerFillVo, 0, len(fills))
	for _, fill := range fills {
		ledgerFills = append(ledgerFills, fill.ToTradeLedgerFillVo())
	}

	return NewTradeLedgerDomain(ledgerFills, spotTradeLedgerWording)
}

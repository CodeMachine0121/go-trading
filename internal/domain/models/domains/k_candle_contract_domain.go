package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// KCandleContractDomain holds one contract K candle and guarantees its own
// invariants. An instance only exists when every rule passed, so there is no
// half-valid contract K candle — and in particular no contract K candle that is
// indistinguishable from a spot one because its mark price never arrived.
type KCandleContractDomain struct {
	kCandle    KCandleDomain
	tradeCount int64
	markOpen   decimal.Decimal
	markHigh   decimal.Decimal
	markLow    decimal.Decimal
	markClose  decimal.Decimal
	// indexLine and premiumIndexLine are the two lines the venue computes beside the
	// traded one. Each answers to its own rules and neither is compared with the
	// traded prices or the mark ones: how far apart they sit is the market's business.
	indexLine        contractPriceLineDomain
	premiumIndexLine contractPriceLineDomain
}

// NewKCandleContractDomain validates the figures against every contract K candle
// rule, judging "in the future" against currentTime.
//
// The rules a contract K candle shares with a spot one are not restated here — they
// are applied by building a spot K candle out of the same figures first. Restating
// them would give the system two lists that merely happen to agree today.
func NewKCandleContractDomain(
	writeDto dto.KCandleContractWriteDto, currentTime time.Time,
) (KCandleContractDomain, error) {
	kCandle, kCandleError := NewKCandleDomain(dto.KCandleWriteDto{
		Symbol:              writeDto.Symbol,
		OpenTime:            writeDto.OpenTime,
		Open:                writeDto.Open,
		High:                writeDto.High,
		Low:                 writeDto.Low,
		Close:               writeDto.Close,
		Volume:              writeDto.Volume,
		QuoteVolume:         writeDto.QuoteVolume,
		TakerBuyBaseVolume:  writeDto.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: writeDto.TakerBuyQuoteVolume,
	}, currentTime)
	if kCandleError != nil {
		return KCandleContractDomain{}, fmt.Errorf("%w: %w", ErrKCandleContractValidation, kCandleError)
	}

	// Every figure is required, which is this record's whole difference from a spot
	// one: it has a single source and that source reports all of them, so a figure
	// that did not arrive is a candle that did not fully arrive.
	// Listed rather than mapped so that a caller who omits two of them is told about
	// the same one every time.
	requiredFigures := []struct {
		name   string
		figure decimal.NullDecimal
	}{
		{"成交額", writeDto.QuoteVolume},
		{"主動買入量", writeDto.TakerBuyBaseVolume},
		{"主動買入額", writeDto.TakerBuyQuoteVolume},
	}
	for _, required := range requiredFigures {
		if !required.figure.Valid {
			return KCandleContractDomain{}, fmt.Errorf(
				"%w: %s不得留白", ErrKCandleContractValidation, required.name)
		}
	}

	if writeDto.TradeCount == nil {
		// Zero is a lawful trade count — a minute in which nothing traded — so a
		// blank one cannot quietly be read as zero.
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 成交筆數不得留白", ErrKCandleContractValidation)
	}
	if *writeDto.TradeCount < 0 {
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 成交筆數不得為負數", ErrKCandleContractValidation)
	}

	markFigures := []decimal.NullDecimal{
		writeDto.MarkOpen, writeDto.MarkHigh, writeDto.MarkLow, writeDto.MarkClose,
	}
	for _, markFigure := range markFigures {
		if !markFigure.Valid {
			// Without it this record is a spot K candle wearing another table's name,
			// which is the one thing keeping the two apart was meant to prevent.
			return KCandleContractDomain{}, fmt.Errorf(
				"%w: 標記價格不得留白", ErrKCandleContractValidation)
		}
		if markFigure.Decimal.IsNegative() {
			return KCandleContractDomain{}, fmt.Errorf(
				"%w: 標記價格不得為負數", ErrKCandleContractValidation)
		}
	}

	// The mark figures answer to their own high-low rule and to nothing else. They
	// are a second price line the venue computes, so how far they sit from the last
	// traded price is the market's business, not a rule this system gets to have an
	// opinion about.
	if writeDto.MarkHigh.Decimal.LessThan(writeDto.MarkLow.Decimal) {
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 標記的最高價不得低於最低價", ErrKCandleContractValidation)
	}

	indexLine, indexError := newContractPriceLineDomain(
		"指數價格", writeDto.IndexOpen, writeDto.IndexHigh, writeDto.IndexLow, writeDto.IndexClose)
	if indexError != nil {
		return KCandleContractDomain{}, indexError
	}
	// An index is a weighted average of spot prices, and no spot price is negative.
	if indexLine.hasNegativeFigure() {
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 指數價格不得為負", ErrKCandleContractValidation)
	}

	// The premium index is a proportion, not a price: a contract trading below its
	// index has a negative one, and that is one of the two ordinary states of the
	// market rather than a broken figure. So it answers to the high-low rule alone.
	premiumIndexLine, premiumIndexError := newContractPriceLineDomain(
		"溢價指數", writeDto.PremiumIndexOpen, writeDto.PremiumIndexHigh,
		writeDto.PremiumIndexLow, writeDto.PremiumIndexClose)
	if premiumIndexError != nil {
		return KCandleContractDomain{}, premiumIndexError
	}

	return KCandleContractDomain{
		kCandle:          kCandle,
		tradeCount:       *writeDto.TradeCount,
		markOpen:         writeDto.MarkOpen.Decimal,
		markHigh:         writeDto.MarkHigh.Decimal,
		markLow:          writeDto.MarkLow.Decimal,
		markClose:        writeDto.MarkClose.Decimal,
		indexLine:        indexLine,
		premiumIndexLine: premiumIndexLine,
	}, nil
}

// ToEntity converts this validated contract K candle into the record shape that is
// stored.
func (kCandleContractDomain KCandleContractDomain) ToEntity() entities.KCandleContract {
	kCandle := kCandleContractDomain.kCandle.ToEntity()

	return entities.KCandleContract{
		Symbol:              kCandle.Symbol,
		OpenTime:            kCandle.OpenTime,
		Open:                kCandle.Open,
		High:                kCandle.High,
		Low:                 kCandle.Low,
		Close:               kCandle.Close,
		Volume:              kCandle.Volume,
		QuoteVolume:         kCandle.QuoteVolume.Decimal,
		TakerBuyBaseVolume:  kCandle.TakerBuyBaseVolume.Decimal,
		TakerBuyQuoteVolume: kCandle.TakerBuyQuoteVolume.Decimal,
		TradeCount:          kCandleContractDomain.tradeCount,
		MarkOpen:            kCandleContractDomain.markOpen,
		MarkHigh:            kCandleContractDomain.markHigh,
		MarkLow:             kCandleContractDomain.markLow,
		MarkClose:           kCandleContractDomain.markClose,
		IndexOpen:           decimal.NewNullDecimal(kCandleContractDomain.indexLine.open),
		IndexHigh:           decimal.NewNullDecimal(kCandleContractDomain.indexLine.high),
		IndexLow:            decimal.NewNullDecimal(kCandleContractDomain.indexLine.low),
		IndexClose:          decimal.NewNullDecimal(kCandleContractDomain.indexLine.close),
		PremiumIndexOpen:    decimal.NewNullDecimal(kCandleContractDomain.premiumIndexLine.open),
		PremiumIndexHigh:    decimal.NewNullDecimal(kCandleContractDomain.premiumIndexLine.high),
		PremiumIndexLow:     decimal.NewNullDecimal(kCandleContractDomain.premiumIndexLine.low),
		PremiumIndexClose:   decimal.NewNullDecimal(kCandleContractDomain.premiumIndexLine.close),
	}
}

package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// KCandleContractDomain only exists when every rule passed, so no contract candle lacks the mark price that distinguishes it from a spot one.
type KCandleContractDomain struct {
	kCandle    KCandleDomain
	tradeCount int64
	markOpen   decimal.Decimal
	markHigh   decimal.Decimal
	markLow    decimal.Decimal
	markClose  decimal.Decimal
	// indexLine and premiumIndexLine are validated on their own and never compared with traded or mark prices.
	indexLine        contractPriceLineDomain
	premiumIndexLine contractPriceLineDomain
}

// NewKCandleContractDomain applies the shared spot rules by building a spot K candle first, then the contract-only rules.
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

	// Unlike spot, the single contract source reports every figure, so a missing one means an incomplete candle; listed so the same omission is always named first.
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
		// Zero is a lawful trade count, so blank cannot be read as zero.
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
			return KCandleContractDomain{}, fmt.Errorf(
				"%w: 標記價格不得留白", ErrKCandleContractValidation)
		}
		if markFigure.Decimal.IsNegative() {
			return KCandleContractDomain{}, fmt.Errorf(
				"%w: 標記價格不得為負數", ErrKCandleContractValidation)
		}
	}

	// Mark prices only need high >= low; their distance from traded prices is the market's business.
	if writeDto.MarkHigh.Decimal.LessThan(writeDto.MarkLow.Decimal) {
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 標記的最高價不得低於最低價", ErrKCandleContractValidation)
	}

	indexLine, indexError := newContractPriceLineDomain(
		"指數價格", writeDto.IndexOpen, writeDto.IndexHigh, writeDto.IndexLow, writeDto.IndexClose)
	if indexError != nil {
		return KCandleContractDomain{}, indexError
	}
	// An index averages spot prices, none of which is negative.
	if indexLine.hasNegativeFigure() {
		return KCandleContractDomain{}, fmt.Errorf(
			"%w: 指數價格不得為負", ErrKCandleContractValidation)
	}

	// The premium index is a proportion that is negative whenever the contract trades below its index, so only high >= low applies.
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

package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// KCandleInterval is the single definition of a stored K candle's length; sources, ingestion, aggregation and session closes all derive from it.
const KCandleInterval = time.Minute

// KCandleIntervalMinutes is the interval in whole minutes, the unit every market source uses.
const KCandleIntervalMinutes = int(KCandleInterval / time.Minute)

// Fails to compile if KCandleInterval is shorter than a minute, which would make sources ask for "0m" candles and silently get nothing.
const _ = uint(KCandleIntervalMinutes - 1)

// KCandleDomain only exists when every rule passed.
type KCandleDomain struct {
	symbol              string
	openTime            time.Time
	open                decimal.Decimal
	high                decimal.Decimal
	low                 decimal.Decimal
	close               decimal.Decimal
	volume              decimal.Decimal
	quoteVolume         decimal.NullDecimal
	takerBuyBaseVolume  decimal.NullDecimal
	takerBuyQuoteVolume decimal.NullDecimal
}

// NewKCandleDomain validates every K candle rule, judging "in the future" against currentTime.
func NewKCandleDomain(writeDto dto.KCandleWriteDto, currentTime time.Time) (KCandleDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(writeDto.Symbol)
	if symbolError != nil {
		return KCandleDomain{}, fmt.Errorf("%w: %w", ErrKCandleValidation, symbolError)
	}

	openTime := writeDto.OpenTime.UTC()
	if !openTime.Truncate(KCandleInterval).Equal(openTime) {
		return KCandleDomain{}, fmt.Errorf(
			"%w: 起始時間必須落在%d分鐘刻度上", ErrKCandleValidation, KCandleIntervalMinutes)
	}

	if openTime.After(currentTime.UTC()) {
		return KCandleDomain{}, fmt.Errorf("%w: 起始時間不得指向未來", ErrKCandleValidation)
	}

	if writeDto.High.LessThan(writeDto.Low) {
		return KCandleDomain{}, fmt.Errorf("%w: 最高價不得低於最低價", ErrKCandleValidation)
	}

	// Unreported optional figures break no rule.
	figures := []OptionalFigureDomain{
		NewOptionalFigureDomain(decimal.NewNullDecimal(writeDto.Open)),
		NewOptionalFigureDomain(decimal.NewNullDecimal(writeDto.High)),
		NewOptionalFigureDomain(decimal.NewNullDecimal(writeDto.Low)),
		NewOptionalFigureDomain(decimal.NewNullDecimal(writeDto.Close)),
		NewOptionalFigureDomain(decimal.NewNullDecimal(writeDto.Volume)),
		NewOptionalFigureDomain(writeDto.QuoteVolume),
		NewOptionalFigureDomain(writeDto.TakerBuyBaseVolume),
		NewOptionalFigureDomain(writeDto.TakerBuyQuoteVolume),
	}
	for _, figure := range figures {
		if figure.IsNegative() {
			return KCandleDomain{}, fmt.Errorf("%w: 價格與成交數字不得為負數", ErrKCandleValidation)
		}
	}

	return KCandleDomain{
		symbol:              tradingSymbol.Value(),
		openTime:            openTime,
		open:                writeDto.Open,
		high:                writeDto.High,
		low:                 writeDto.Low,
		close:               writeDto.Close,
		volume:              writeDto.Volume,
		quoteVolume:         writeDto.QuoteVolume,
		takerBuyBaseVolume:  writeDto.TakerBuyBaseVolume,
		takerBuyQuoteVolume: writeDto.TakerBuyQuoteVolume,
	}, nil
}

func (kCandleDomain KCandleDomain) ToEntity() entities.KCandle {
	return entities.KCandle{
		Symbol:              kCandleDomain.symbol,
		OpenTime:            kCandleDomain.openTime,
		Open:                kCandleDomain.open,
		High:                kCandleDomain.high,
		Low:                 kCandleDomain.low,
		Close:               kCandleDomain.close,
		Volume:              kCandleDomain.volume,
		QuoteVolume:         kCandleDomain.quoteVolume,
		TakerBuyBaseVolume:  kCandleDomain.takerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleDomain.takerBuyQuoteVolume,
	}
}

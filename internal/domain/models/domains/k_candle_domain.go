package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// kCandleIntervalMinutes is how many minutes one K candle covers. It is the single
// place this length is written down; widening the feature to other lengths starts here.
const kCandleIntervalMinutes = 5

// KCandleDomain holds one K candle and guarantees its own invariants. An instance
// only exists when every rule passed, so there is no half-valid K candle.
type KCandleDomain struct {
	symbol              string
	openTime            time.Time
	open                decimal.Decimal
	high                decimal.Decimal
	low                 decimal.Decimal
	close               decimal.Decimal
	volume              decimal.Decimal
	quoteVolume         decimal.Decimal
	takerBuyBaseVolume  decimal.Decimal
	takerBuyQuoteVolume decimal.Decimal
}

// NewKCandleDomain validates the figures against every K candle rule, judging
// "in the future" against currentTime.
func NewKCandleDomain(writeDto dto.KCandleWriteDto, currentTime time.Time) (KCandleDomain, error) {
	if writeDto.Symbol == "" {
		return KCandleDomain{}, fmt.Errorf("%w: 必須指定交易標的", ErrKCandleValidation)
	}

	openTime := writeDto.OpenTime.UTC()
	isOnInterval := openTime.Minute()%kCandleIntervalMinutes == 0 &&
		openTime.Second() == 0 &&
		openTime.Nanosecond() == 0
	if !isOnInterval {
		return KCandleDomain{}, fmt.Errorf(
			"%w: 起始時間必須落在%d分鐘刻度上", ErrKCandleValidation, kCandleIntervalMinutes)
	}

	if openTime.After(currentTime.UTC()) {
		return KCandleDomain{}, fmt.Errorf("%w: 起始時間不得指向未來", ErrKCandleValidation)
	}

	if writeDto.High.LessThan(writeDto.Low) {
		return KCandleDomain{}, fmt.Errorf("%w: 最高價不得低於最低價", ErrKCandleValidation)
	}

	figures := []decimal.Decimal{
		writeDto.Open, writeDto.High, writeDto.Low, writeDto.Close,
		writeDto.Volume, writeDto.QuoteVolume,
		writeDto.TakerBuyBaseVolume, writeDto.TakerBuyQuoteVolume,
	}
	for _, figure := range figures {
		if figure.IsNegative() {
			return KCandleDomain{}, fmt.Errorf("%w: 價格與成交數字不得為負數", ErrKCandleValidation)
		}
	}

	return KCandleDomain{
		symbol:              writeDto.Symbol,
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

// ToEntity converts this validated K candle into the record shape that is stored.
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

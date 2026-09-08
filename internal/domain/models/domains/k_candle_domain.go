package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// KCandleInterval is how long one K candle covers, and it is the only place the
// system writes that length down. Changing the granularity is changing this line:
// the market sources spell their own requests from it, the ingestion round takes its
// interval from it, aggregation counts source candles with it, and a market's last
// candle of the session is its closing time less this.
//
// It is exported for exactly that reason. Left unexported, every one of those places
// had to write the length out again and merely happen to agree — and a place that
// forgot would store candles of a length nothing in the system could detect.
const KCandleInterval = time.Minute

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
	quoteVolume         decimal.NullDecimal
	takerBuyBaseVolume  decimal.NullDecimal
	takerBuyQuoteVolume decimal.NullDecimal
}

// NewKCandleDomain validates the figures against every K candle rule, judging
// "in the future" against currentTime.
func NewKCandleDomain(writeDto dto.KCandleWriteDto, currentTime time.Time) (KCandleDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(writeDto.Symbol)
	if symbolError != nil {
		return KCandleDomain{}, fmt.Errorf("%w: %w", ErrKCandleValidation, symbolError)
	}

	// Truncating says the whole rule in one line — the minutes, the seconds and
	// everything finer at once — and it says it for whatever length the system runs
	// on rather than only for lengths that divide an hour into whole minutes.
	openTime := writeDto.OpenTime.UTC()
	if !openTime.Truncate(KCandleInterval).Equal(openTime) {
		return KCandleDomain{}, fmt.Errorf(
			"%w: 起始時間必須落在%d分鐘刻度上",
			ErrKCandleValidation, int(KCandleInterval/time.Minute))
	}

	if openTime.After(currentTime.UTC()) {
		return KCandleDomain{}, fmt.Errorf("%w: 起始時間不得指向未來", ErrKCandleValidation)
	}

	if writeDto.High.LessThan(writeDto.Low) {
		return KCandleDomain{}, fmt.Errorf("%w: 最高價不得低於最低價", ErrKCandleValidation)
	}

	// Every figure is judged the same way whether or not the market reports it: a
	// figure that was never reported breaks no rule, which is the one difference and
	// it is stated once, in the figure itself.
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

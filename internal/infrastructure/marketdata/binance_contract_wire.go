package marketdata

import (
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// tradeCountIndex is where the number of trades sits in the source's positional
// array. The spot side does not read it — a spot K candle does not record it — so it
// is named here rather than beside the shared indexes.
const tradeCountIndex = 8

// markPriceFigures are the four prices a mark price answer actually carries. The
// same answer also carries volume, turnover and taker volumes, and every one of them
// is the string "0".
//
// **Those zeros are placeholders, not readings.** A minute in which nothing traded
// genuinely has a volume of zero, so copying them across would put candles into
// storage that nobody could later tell apart from real ones. They are dropped here,
// at the only place that knows they are not numbers.
type markPriceFigures struct {
	open  decimal.Decimal
	high  decimal.Decimal
	low   decimal.Decimal
	close decimal.Decimal
}

// toContractMarketKCandleVo turns the traded half of a contract candle into the shape
// the domain accepts, leaving the mark figures absent for the merge to fill in.
// Nothing is judged here — the contract K candle rules are applied further in.
// The three optional figures are read without checking whether they arrived: the
// shared conversion above fills all of them from the same answer, so on this venue
// they are present whenever it succeeded.
func (kLine binanceKLine) toContractMarketKCandleVo(symbol string) (vo.ContractMarketKCandleVo, error) {
	marketKCandle, convertError := kLine.toMarketKCandleVo(symbol)
	if convertError != nil {
		return vo.ContractMarketKCandleVo{}, convertError
	}

	tradeCount, tradeCountError := kLine.tradeCount()
	if tradeCountError != nil {
		return vo.ContractMarketKCandleVo{}, tradeCountError
	}

	return vo.ContractMarketKCandleVo{
		Symbol:              marketKCandle.Symbol,
		OpenTime:            marketKCandle.OpenTime,
		Open:                marketKCandle.Open,
		High:                marketKCandle.High,
		Low:                 marketKCandle.Low,
		Close:               marketKCandle.Close,
		Volume:              marketKCandle.Volume,
		QuoteVolume:         marketKCandle.QuoteVolume.Decimal,
		TakerBuyBaseVolume:  marketKCandle.TakerBuyBaseVolume.Decimal,
		TakerBuyQuoteVolume: marketKCandle.TakerBuyQuoteVolume.Decimal,
		TradeCount:          tradeCount,
	}, nil
}

// toMarkPriceFigures reads only the four prices out of a mark price answer.
func (kLine binanceKLine) toMarkPriceFigures() (markPriceFigures, error) {
	if len(kLine) < kLineFieldCount {
		return markPriceFigures{}, fmt.Errorf(
			"mark price from market source has %d fields, expected at least %d",
			len(kLine), kLineFieldCount)
	}

	figures, figureError := kLine.figures()
	if figureError != nil {
		return markPriceFigures{}, figureError
	}

	return markPriceFigures{
		open:  figures[openIndex],
		high:  figures[highIndex],
		low:   figures[lowIndex],
		close: figures[closeIndex],
	}, nil
}

// tradeCount reads how many trades the minute held, which the source states as a
// bare number rather than as a quoted decimal like every figure beside it.
//
// The position is not bounds-checked here because it cannot be out of bounds: this is
// only reached once the shared conversion has vouched for the row's length, and that
// length is larger than this position.
func (kLine binanceKLine) tradeCount() (int64, error) {
	var tradeCount int64
	if decodeError := json.Unmarshal(kLine[tradeCountIndex], &tradeCount); decodeError != nil {
		return 0, fmt.Errorf("read trade count from market source: %w", decodeError)
	}

	return tradeCount, nil
}

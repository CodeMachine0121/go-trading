package marketdata

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// Binance reports a K candle as a positional array rather than named fields, so
// these indexes are the whole schema. They are the one place the wire layout is
// written down; a source that reorders its array is only visible here.
const (
	openTimeIndex            = 0
	openIndex                = 1
	highIndex                = 2
	lowIndex                 = 3
	closeIndex               = 4
	volumeIndex              = 5
	quoteVolumeIndex         = 7
	takerBuyBaseVolumeIndex  = 9
	takerBuyQuoteVolumeIndex = 10
	kLineFieldCount          = 11
)

// binanceKLine is one K candle exactly as it arrives: a mixed array of a number and
// quoted decimals. Raw messages keep every element typed at the point it is read,
// so no loosely typed value ever exists.
type binanceKLine []json.RawMessage

// toMarketKCandleVo turns the positional array into the shape the domain accepts.
// Nothing is judged here — the K candle rules are applied further in.
func (kLine binanceKLine) toMarketKCandleVo(symbol string) (vo.MarketKCandleVo, error) {
	if len(kLine) < kLineFieldCount {
		return vo.MarketKCandleVo{}, fmt.Errorf(
			"k candle from market source has %d fields, expected at least %d", len(kLine), kLineFieldCount)
	}

	openTime, openTimeError := kLine.openTime()
	if openTimeError != nil {
		return vo.MarketKCandleVo{}, openTimeError
	}

	figures, figureError := kLine.figures()
	if figureError != nil {
		return vo.MarketKCandleVo{}, figureError
	}

	return vo.MarketKCandleVo{
		Symbol:              symbol,
		OpenTime:            openTime,
		Open:                figures[openIndex],
		High:                figures[highIndex],
		Low:                 figures[lowIndex],
		Close:               figures[closeIndex],
		Volume:              figures[volumeIndex],
		QuoteVolume:         figures[quoteVolumeIndex],
		TakerBuyBaseVolume:  figures[takerBuyBaseVolumeIndex],
		TakerBuyQuoteVolume: figures[takerBuyQuoteVolumeIndex],
	}, nil
}

// openTime reads the open time, which the source states in milliseconds.
func (kLine binanceKLine) openTime() (time.Time, error) {
	var openTimeMilliseconds int64
	if decodeError := json.Unmarshal(kLine[openTimeIndex], &openTimeMilliseconds); decodeError != nil {
		return time.Time{}, fmt.Errorf("read open time from market source: %w", decodeError)
	}

	return time.UnixMilli(openTimeMilliseconds).UTC(), nil
}

// figures reads every quoted decimal the domain needs, keyed by its own index so
// that a caller reads them under the same names the wire layout uses.
func (kLine binanceKLine) figures() (map[int]decimal.Decimal, error) {
	figures := make(map[int]decimal.Decimal, 8)
	for _, index := range []int{
		openIndex, highIndex, lowIndex, closeIndex,
		volumeIndex, quoteVolumeIndex, takerBuyBaseVolumeIndex, takerBuyQuoteVolumeIndex,
	} {
		var quotedFigure string
		if decodeError := json.Unmarshal(kLine[index], &quotedFigure); decodeError != nil {
			return nil, fmt.Errorf("read figure at position %d from market source: %w", index, decodeError)
		}

		figure, parseError := decimal.NewFromString(quotedFigure)
		if parseError != nil {
			return nil, fmt.Errorf("read figure at position %d from market source: %w", index, parseError)
		}
		figures[index] = figure
	}

	return figures, nil
}

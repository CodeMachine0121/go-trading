package marketdata

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// Binance K candles are positional arrays, so these indexes are the entire wire schema.
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

// binanceKLine keeps elements as raw messages so each is decoded to a concrete type where read.
type binanceKLine []json.RawMessage

// toMarketKCandleVo converts without validation; K candle rules are applied later.
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
		QuoteVolume:         decimal.NewNullDecimal(figures[quoteVolumeIndex]),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(figures[takerBuyBaseVolumeIndex]),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(figures[takerBuyQuoteVolumeIndex]),
	}, nil
}

// openTime reads the open time, given in milliseconds.
func (kLine binanceKLine) openTime() (time.Time, error) {
	var openTimeMilliseconds int64
	if decodeError := json.Unmarshal(kLine[openTimeIndex], &openTimeMilliseconds); decodeError != nil {
		return time.Time{}, fmt.Errorf("read open time from market source: %w", decodeError)
	}

	return time.UnixMilli(openTimeMilliseconds).UTC(), nil
}

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

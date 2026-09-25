package marketdata

import (
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// tradeCountIndex is contract-only because spot K candles do not record the trade count.
const tradeCountIndex = 8

// priceLineFigures keeps only the four prices of a mark, index or premium answer; its volume fields are placeholder "0"s that would be indistinguishable from real zero volume.
type priceLineFigures struct {
	open  decimal.Decimal
	high  decimal.Decimal
	low   decimal.Decimal
	close decimal.Decimal
}

func (figures priceLineFigures) toNullDecimals() (
	decimal.NullDecimal, decimal.NullDecimal, decimal.NullDecimal, decimal.NullDecimal,
) {
	return decimal.NewNullDecimal(figures.open), decimal.NewNullDecimal(figures.high),
		decimal.NewNullDecimal(figures.low), decimal.NewNullDecimal(figures.close)
}

// toContractMarketKCandleVo converts the traded klines, leaving mark, index and premium figures absent for the merge to fill; the optional volumes are always present on this venue.
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

func (kLine binanceKLine) toPriceLineFigures() (priceLineFigures, error) {
	if len(kLine) < kLineFieldCount {
		return priceLineFigures{}, fmt.Errorf(
			"price line from market source has %d fields, expected at least %d",
			len(kLine), kLineFieldCount)
	}

	figures, figureError := kLine.figures()
	if figureError != nil {
		return priceLineFigures{}, figureError
	}

	return priceLineFigures{
		open:  figures[openIndex],
		high:  figures[highIndex],
		low:   figures[lowIndex],
		close: figures[closeIndex],
	}, nil
}

// tradeCount reads a bare JSON number; the index needs no bounds check because the shared conversion already validated the row length.
func (kLine binanceKLine) tradeCount() (int64, error) {
	var tradeCount int64
	if decodeError := json.Unmarshal(kLine[tradeCountIndex], &tradeCount); decodeError != nil {
		return 0, fmt.Errorf("read trade count from market source: %w", decodeError)
	}

	return tradeCount, nil
}

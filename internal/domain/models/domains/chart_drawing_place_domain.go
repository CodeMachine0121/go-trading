package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingPlaceDomain is one board's address: the owner, spot or contract, and an upper-cased symbol.
type ChartDrawingPlaceDomain struct {
	ownerID uint
	market  vo.TradableMarketVo
	symbol  string
}

func NewChartDrawingPlaceDomain(placeDto dto.ChartDrawingBoardPlaceDto) (ChartDrawingPlaceDomain, error) {
	market := vo.TradableMarketVo(placeDto.Market)
	if market != vo.TradableMarketSpot && market != vo.TradableMarketContract {
		return ChartDrawingPlaceDomain{}, fmt.Errorf(
			"%w: 市場只能是現貨（spot）或合約（contract），收到的是「%s」", ErrChartDrawingValidation, placeDto.Market)
	}

	tradingSymbol, symbolError := NewTradingSymbolDomain(placeDto.Symbol)
	if symbolError != nil {
		return ChartDrawingPlaceDomain{}, fmt.Errorf("%w: %w", ErrChartDrawingValidation, symbolError)
	}

	return ChartDrawingPlaceDomain{ownerID: placeDto.OwnerID, market: market, symbol: tradingSymbol.Value()}, nil
}

func (chartDrawingPlaceDomain ChartDrawingPlaceDomain) OwnerID() uint {
	return chartDrawingPlaceDomain.ownerID
}

func (chartDrawingPlaceDomain ChartDrawingPlaceDomain) Market() string {
	return string(chartDrawingPlaceDomain.market)
}

func (chartDrawingPlaceDomain ChartDrawingPlaceDomain) Symbol() string {
	return chartDrawingPlaceDomain.symbol
}

func (chartDrawingPlaceDomain ChartDrawingPlaceDomain) IsContract() bool {
	return chartDrawingPlaceDomain.market == vo.TradableMarketContract
}

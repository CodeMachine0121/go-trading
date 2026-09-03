package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// KCandleQueryDomain holds one time-range query and guarantees its own invariants.
type KCandleQueryDomain struct {
	symbol    string
	startTime time.Time
	endTime   time.Time
}

// NewKCandleQueryDomain validates the query against every query rule. The start and
// end times are deliberately not required to sit on a K candle interval boundary.
func NewKCandleQueryDomain(queryDto dto.KCandleQueryDto) (KCandleQueryDomain, error) {
	tradingSymbol, symbolError := NewTradingSymbolDomain(queryDto.Symbol)
	if symbolError != nil {
		return KCandleQueryDomain{}, fmt.Errorf("%w: %w", ErrKCandleValidation, symbolError)
	}

	startTime := queryDto.StartTime.UTC()
	endTime := queryDto.EndTime.UTC()
	if endTime.Before(startTime) {
		return KCandleQueryDomain{}, fmt.Errorf("%w: 結束時間不得早於開始時間", ErrKCandleValidation)
	}

	return KCandleQueryDomain{
		symbol: tradingSymbol.Value(), startTime: startTime, endTime: endTime,
	}, nil
}

func (kCandleQueryDomain KCandleQueryDomain) Symbol() string {
	return kCandleQueryDomain.symbol
}

func (kCandleQueryDomain KCandleQueryDomain) StartTime() time.Time {
	return kCandleQueryDomain.startTime
}

func (kCandleQueryDomain KCandleQueryDomain) EndTime() time.Time {
	return kCandleQueryDomain.endTime
}

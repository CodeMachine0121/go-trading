package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type KCandleQueryDomain struct {
	symbol    string
	startTime time.Time
	endTime   time.Time
}

// NewKCandleQueryDomain deliberately does not require start and end to sit on a K candle boundary.
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

// MinuteCapacity bounds a read of the stretch's stored minutes, both ends included.
func (kCandleQueryDomain KCandleQueryDomain) MinuteCapacity() int {
	return int(kCandleQueryDomain.endTime.Sub(kCandleQueryDomain.startTime)/KCandleInterval) + 1
}

// requireOrderFlowReach refuses a replay whose whole stretch, warm-up included, spans more days of order flow than one run may hold, so it is refused when submitted rather than after it was queued; a script declaring no price step is never refused.
func (kCandleQueryDomain KCandleQueryDomain) requireOrderFlowReach(parameters StrategyScriptParametersDomain) error {
	if _, declaresOrderFlow := parameters.OrderFlowPriceStep(); !declaresOrderFlow {
		return nil
	}

	reachError := OrderFlowReachDomain{start: kCandleQueryDomain.startTime, end: kCandleQueryDomain.endTime}.requireWithinDays()
	if reachError != nil {
		return fmt.Errorf("%w: %w", ErrBacktestValidation, reachError)
	}

	return nil
}

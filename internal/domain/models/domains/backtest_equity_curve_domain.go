package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestEquityCurveDomain records equity per replayed candle and tracks the maximum drawdown incrementally, since it depends on point order.
type BacktestEquityCurveDomain struct {
	initialCapital decimal.Decimal
	points         []vo.EquityPointVo
	// peakEquity starts at the initial capital so a replay that only falls still reports its drawdown.
	peakEquity      decimal.Decimal
	maximumDrawdown float64
}

func NewBacktestEquityCurveDomain(initialCapital decimal.Decimal) *BacktestEquityCurveDomain {
	return &BacktestEquityCurveDomain{
		initialCapital: initialCapital,
		points:         make([]vo.EquityPointVo, 0),
		peakEquity:     initialCapital,
	}
}

// Record is called exactly once per replayed candle, keeping the curve aligned with the candles.
func (backtestEquityCurveDomain *BacktestEquityCurveDomain) Record(
	candleTime time.Time, equity decimal.Decimal,
) {
	backtestEquityCurveDomain.points = append(
		backtestEquityCurveDomain.points,
		vo.EquityPointVo{OpenTime: candleTime, Equity: equity})

	if equity.GreaterThan(backtestEquityCurveDomain.peakEquity) {
		backtestEquityCurveDomain.peakEquity = equity
	}

	// A non-positive peak has no meaningful drawdown.
	if !backtestEquityCurveDomain.peakEquity.IsPositive() {
		return
	}

	drawdown, _ := backtestEquityCurveDomain.peakEquity.Sub(equity).
		Div(backtestEquityCurveDomain.peakEquity).Float64()
	backtestEquityCurveDomain.maximumDrawdown = max(
		backtestEquityCurveDomain.maximumDrawdown, drawdown)
}

func (backtestEquityCurveDomain *BacktestEquityCurveDomain) PointDtos() []dto.EquityPointDto {
	equityPointDtos := make([]dto.EquityPointDto, 0, len(backtestEquityCurveDomain.points))
	for _, equityPoint := range backtestEquityCurveDomain.points {
		equityPointDtos = append(equityPointDtos, equityPoint.ToDto())
	}

	return equityPointDtos
}

func (backtestEquityCurveDomain *BacktestEquityCurveDomain) MaximumDrawdown() float64 {
	return backtestEquityCurveDomain.maximumDrawdown
}

// FinalEquity is the initial capital when nothing was recorded.
func (backtestEquityCurveDomain *BacktestEquityCurveDomain) FinalEquity() decimal.Decimal {
	if len(backtestEquityCurveDomain.points) == 0 {
		return backtestEquityCurveDomain.initialCapital
	}

	return backtestEquityCurveDomain.points[len(backtestEquityCurveDomain.points)-1].Equity
}

func (backtestEquityCurveDomain *BacktestEquityCurveDomain) TotalReturnRate() float64 {
	if !backtestEquityCurveDomain.initialCapital.IsPositive() {
		return 0
	}

	totalReturnRate, _ := backtestEquityCurveDomain.FinalEquity().
		Sub(backtestEquityCurveDomain.initialCapital).
		Div(backtestEquityCurveDomain.initialCapital).Float64()

	return totalReturnRate
}

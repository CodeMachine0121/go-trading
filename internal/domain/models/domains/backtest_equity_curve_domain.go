package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestEquityCurveDomain is what the account was worth at the close of every candle
// replayed, and the two things that can only be read off the whole shape of it: where
// it ended, and the worst fall from a peak along the way.
//
// The worst fall is kept as the curve grows rather than worked out afterwards, because
// it is the only reading that needs the points in order — and computing it here means
// nobody downstream can compute it from a curve that has been reordered or trimmed.
type BacktestEquityCurveDomain struct {
	initialCapital decimal.Decimal
	points         []vo.EquityPointVo
	// peakEquity starts at the initial capital rather than at the first point. A
	// replay that only ever falls had its high before the first candle, and seeding
	// from the first point would report no drawdown at all for exactly the strategy
	// that most deserves one.
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

// Record puts down what the account was worth once one candle had closed. Every
// replayed candle records exactly once, which is what keeps the curve and the candles
// the same length.
func (backtestEquityCurveDomain *BacktestEquityCurveDomain) Record(
	candleTime time.Time, equity decimal.Decimal,
) {
	backtestEquityCurveDomain.points = append(
		backtestEquityCurveDomain.points,
		vo.EquityPointVo{OpenTime: candleTime, Equity: equity})

	if equity.GreaterThan(backtestEquityCurveDomain.peakEquity) {
		backtestEquityCurveDomain.peakEquity = equity
	}

	// A peak of zero or less has no fall to measure against — dividing by it would ask
	// how far below nothing the account has gone, which is not a question.
	if !backtestEquityCurveDomain.peakEquity.IsPositive() {
		return
	}

	drawdown, _ := backtestEquityCurveDomain.peakEquity.Sub(equity).
		Div(backtestEquityCurveDomain.peakEquity).Float64()
	backtestEquityCurveDomain.maximumDrawdown = max(
		backtestEquityCurveDomain.maximumDrawdown, drawdown)
}

// PointDtos are every point recorded, in the order they were recorded, in the shape
// they leave the domain in.
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

// FinalEquity is where the curve ended. A curve with no points at all ended where it
// started, which is the only honest answer for a replay that recorded nothing.
func (backtestEquityCurveDomain *BacktestEquityCurveDomain) FinalEquity() decimal.Decimal {
	if len(backtestEquityCurveDomain.points) == 0 {
		return backtestEquityCurveDomain.initialCapital
	}

	return backtestEquityCurveDomain.points[len(backtestEquityCurveDomain.points)-1].Equity
}

// TotalReturnRate is what the whole replay made, as a share of what it started with.
func (backtestEquityCurveDomain *BacktestEquityCurveDomain) TotalReturnRate() float64 {
	if !backtestEquityCurveDomain.initialCapital.IsPositive() {
		return 0
	}

	totalReturnRate, _ := backtestEquityCurveDomain.FinalEquity().
		Sub(backtestEquityCurveDomain.initialCapital).
		Div(backtestEquityCurveDomain.initialCapital).Float64()

	return totalReturnRate
}

package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// BacktestExitLevelsDomain holds a replay's validated stop-loss and take-profit distances; its zero value simulates no exits.
type BacktestExitLevelsDomain struct {
	// Zero disables each exit independently.
	stopLoss   decimal.Decimal
	takeProfit decimal.Decimal
}

// NewBacktestExitLevelsDomain reuses the bot position plan's validation so both forms give identical messages.
func NewBacktestExitLevelsDomain(
	stopLoss decimal.Decimal, takeProfit decimal.Decimal,
) (BacktestExitLevelsDomain, error) {
	if stopLossError := validatedDistance(stopLoss, "停損距離"); stopLossError != nil {
		return BacktestExitLevelsDomain{}, stopLossError
	}

	if takeProfitError := validatedDistance(takeProfit, "停利距離"); takeProfitError != nil {
		return BacktestExitLevelsDomain{}, takeProfitError
	}

	return BacktestExitLevelsDomain{stopLoss: stopLoss, takeProfit: takeProfit}, nil
}

// PricesFrom places a long position's stop below and target above the entry; whether a candle hits them is BacktestPositionDomain.ExitOn's job.
func (backtestExitLevelsDomain BacktestExitLevelsDomain) PricesFrom(
	entryPrice decimal.Decimal,
) vo.ExitPricesVo {
	exitPricesVo := vo.ExitPricesVo{
		HasStopLoss:   backtestExitLevelsDomain.stopLoss.IsPositive(),
		HasTakeProfit: backtestExitLevelsDomain.takeProfit.IsPositive(),
	}

	if exitPricesVo.HasStopLoss {
		exitPricesVo.StopLossPrice = entryPrice.Sub(
			portionOf(entryPrice, backtestExitLevelsDomain.stopLoss))
	}

	if exitPricesVo.HasTakeProfit {
		exitPricesVo.TakeProfitPrice = entryPrice.Add(
			portionOf(entryPrice, backtestExitLevelsDomain.takeProfit))
	}

	return exitPricesVo
}

// PricesFacing mirrors the levels for a short position, whose stop sits above the entry.
func (backtestExitLevelsDomain BacktestExitLevelsDomain) PricesFacing(
	direction vo.PositionDirectionVo, entryPrice decimal.Decimal,
) vo.ExitPricesVo {
	if direction != vo.PositionDirectionShort {
		return backtestExitLevelsDomain.PricesFrom(entryPrice)
	}

	exitPricesVo := vo.ExitPricesVo{
		HasStopLoss:   backtestExitLevelsDomain.stopLoss.IsPositive(),
		HasTakeProfit: backtestExitLevelsDomain.takeProfit.IsPositive(),
	}

	if exitPricesVo.HasStopLoss {
		exitPricesVo.StopLossPrice = entryPrice.Add(
			portionOf(entryPrice, backtestExitLevelsDomain.stopLoss))
	}

	if exitPricesVo.HasTakeProfit {
		exitPricesVo.TakeProfitPrice = entryPrice.Sub(
			portionOf(entryPrice, backtestExitLevelsDomain.takeProfit))
	}

	return exitPricesVo
}

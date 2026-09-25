package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_indicator_script_proxy.go -destination=mocks/mock_i_indicator_script_proxy.go -package=mocks

// IIndicatorScriptProxy runs an indicator script over K candles and returns one value per indicator name in the declared kind.
type IIndicatorScriptProxy interface {
	Execute(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		kCandles []vo.KCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) (map[string]vo.IndicatorValueVo, error)
	// ExecuteForEachCandle runs the script once per candle over the prefix up to it, compiling once so replays stay affordable; the first failure aborts with no partial result.
	ExecuteForEachCandle(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		kCandles []vo.KCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) ([]map[string]vo.IndicatorValueVo, error)
}

package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_indicator_script_proxy.go -destination=mocks/mock_i_indicator_script_proxy.go -package=mocks

// IIndicatorScriptProxy runs one user-written indicator script over the given K
// candles and hands back one value per indicator name, in the declared kind.
// Everything about how the script is run — the environment, what it may reach, how
// its shape is checked, how failures are caught — lives behind this contract.
type IIndicatorScriptProxy interface {
	Execute(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		kCandles []vo.KCandleVo,
		parameters domains.StrategyParametersDomain,
	) (map[string]vo.IndicatorValueVo, error)
	// ExecuteForEachCandle runs one script once per K candle: the nth run sees the
	// candles from the first up to and including the nth, and the results come back
	// in that same order, one set per candle.
	//
	// It is not a loop the caller could have written. Reading a script and running it
	// are two different costs, and only the side that owns the running can pay the
	// first one once and the second one many times — which is the difference between
	// a replay that finishes and one that reads the same script a thousand times.
	//
	// The first failure ends everything with no partial result: half a replay is not
	// a shorter replay, it is a wrong one.
	ExecuteForEachCandle(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		kCandles []vo.KCandleVo,
		parameters domains.StrategyParametersDomain,
	) ([]map[string]vo.IndicatorValueVo, error)
}

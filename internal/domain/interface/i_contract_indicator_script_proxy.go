package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_indicator_script_proxy.go -destination=mocks/mock_i_contract_indicator_script_proxy.go -package=mocks

// IContractIndicatorScriptProxy runs one user-written indicator script over perpetual
// contract bars and hands back one value per indicator name, in the declared kind.
//
// It is a contract of its own rather than IIndicatorScriptProxy with a wider
// argument, because the data a script is fed is part of what it is: a script written
// for contract bars has an entry point no spot K candle can be handed to, and one
// interface taking either would have to be told which one it got.
type IContractIndicatorScriptProxy interface {
	Execute(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		contractKCandles []vo.ContractKCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) (map[string]vo.IndicatorValueVo, error)
	// ExecuteForEachCandle runs one script once per contract bar: the nth run sees the
	// bars from the first up to and including the nth, and the results come back in
	// that same order, one set per bar. The first failure ends everything with no
	// partial result.
	ExecuteForEachCandle(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		contractKCandles []vo.ContractKCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) ([]map[string]vo.IndicatorValueVo, error)
}

package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_indicator_script_proxy.go -destination=mocks/mock_i_contract_indicator_script_proxy.go -package=mocks

// IContractIndicatorScriptProxy runs an indicator script over contract bars; it is separate from IIndicatorScriptProxy because contract scripts have a different entry point.
type IContractIndicatorScriptProxy interface {
	Execute(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		contractKCandles []vo.ContractKCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) (map[string]vo.IndicatorValueVo, error)
	// ExecuteForEachCandle runs the script once per bar over the prefix up to that bar, in order; the first failure aborts with no partial result.
	ExecuteForEachCandle(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		contractKCandles []vo.ContractKCandleVo,
		parameters domains.StrategyScriptParametersDomain,
	) ([]map[string]vo.IndicatorValueVo, error)
}

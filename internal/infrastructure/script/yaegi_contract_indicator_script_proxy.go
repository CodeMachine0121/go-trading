package script

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiContractIndicatorScriptProxy runs indicator scripts fed perpetual contract bars.
type YaegiContractIndicatorScriptProxy struct {
	compartment indicatorScriptCompartment[vo.ContractKCandleVo]
}

func NewYaegiContractIndicatorScriptProxy(isolation IndicatorScriptIsolation) *YaegiContractIndicatorScriptProxy {
	return &YaegiContractIndicatorScriptProxy{
		compartment: indicatorScriptCompartment[vo.ContractKCandleVo]{
			isolation: isolation,
			input:     contractIndicatorScriptInput,
		},
	}
}

func (yaegiContractIndicatorScriptProxy *YaegiContractIndicatorScriptProxy) Execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	contractKCandles []vo.ContractKCandleVo,
	parameters domains.StrategyScriptParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	return yaegiContractIndicatorScriptProxy.compartment.execute(
		executionContext, script, resultType, contractKCandles, parameters)
}

// ExecuteForEachCandle replays the script once per contract bar over a growing prefix, reading it once.
func (yaegiContractIndicatorScriptProxy *YaegiContractIndicatorScriptProxy) ExecuteForEachCandle(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	contractKCandles []vo.ContractKCandleVo,
	parameters domains.StrategyScriptParametersDomain,
) ([]map[string]vo.IndicatorValueVo, error) {
	return yaegiContractIndicatorScriptProxy.compartment.executeForEachElement(
		executionContext, script, resultType, contractKCandles, parameters)
}

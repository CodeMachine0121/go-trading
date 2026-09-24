package script

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiContractIndicatorScriptProxy runs contract indicator scripts — scripts fed
// perpetual contract bars — with the same embedded interpreter, sandbox, allowance and
// compartment spot scripts run under.
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

// Execute runs the script over the contract bars and collects its values in the
// declared kind, with no partial result on any failure.
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

// ExecuteForEachCandle runs the same script once per contract bar over a growing
// stretch, reading the script once for the whole replay.
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

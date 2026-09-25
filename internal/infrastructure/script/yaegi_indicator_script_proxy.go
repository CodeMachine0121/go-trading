package script

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiIndicatorScriptProxy runs spot indicator scripts, fed K candles, each in its own compartment.
type YaegiIndicatorScriptProxy struct {
	compartment indicatorScriptCompartment[vo.KCandleVo]
}

func NewYaegiIndicatorScriptProxy(isolation IndicatorScriptIsolation) *YaegiIndicatorScriptProxy {
	return &YaegiIndicatorScriptProxy{
		compartment: indicatorScriptCompartment[vo.KCandleVo]{
			isolation: isolation,
			input:     spotIndicatorScriptInput,
		},
	}
}

func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) Execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
	parameters domains.StrategyScriptParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	return yaegiIndicatorScriptProxy.compartment.execute(executionContext, script, resultType, kCandles, parameters)
}

// ExecuteForEachCandle replays the script once per K candle over a growing prefix, reading it once.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) ExecuteForEachCandle(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
	parameters domains.StrategyScriptParametersDomain,
) ([]map[string]vo.IndicatorValueVo, error) {
	return yaegiIndicatorScriptProxy.compartment.executeForEachElement(
		executionContext, script, resultType, kCandles, parameters)
}

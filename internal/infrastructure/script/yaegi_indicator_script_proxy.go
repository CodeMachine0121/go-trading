package script

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiIndicatorScriptProxy runs spot indicator scripts — scripts fed K candles —
// with an embedded Go interpreter, each run in a compartment of its own. How a script
// is run is the runner's and where it runs is the compartment's; this proxy only says
// that a spot script is fed K candles.
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

// Execute runs the script over the K candles and collects its values in the declared
// kind, with no partial result on any failure.
func (yaegiIndicatorScriptProxy *YaegiIndicatorScriptProxy) Execute(
	executionContext context.Context,
	script string,
	resultType domains.IndicatorResultTypeDomain,
	kCandles []vo.KCandleVo,
	parameters domains.StrategyScriptParametersDomain,
) (map[string]vo.IndicatorValueVo, error) {
	return yaegiIndicatorScriptProxy.compartment.execute(executionContext, script, resultType, kCandles, parameters)
}

// ExecuteForEachCandle runs the same script once per K candle: the nth run sees the
// candles from the first up to and including the nth, and the results come back in
// that same order, one set per candle. The script is read once for the whole replay.
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

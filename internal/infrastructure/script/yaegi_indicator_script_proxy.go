package script

import (
	"context"
	"reflect"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiIndicatorScriptProxy runs spot indicator scripts — scripts fed K candles —
// with an embedded Go interpreter, giving up on any script that outlives its
// allowance. How a script is run is the runner's; this proxy only says what a spot
// script is fed and what it may name.
type YaegiIndicatorScriptProxy struct {
	runner indicatorScriptRunner[vo.KCandleVo]
}

func NewYaegiIndicatorScriptProxy(executionTimeout time.Duration) *YaegiIndicatorScriptProxy {
	return &YaegiIndicatorScriptProxy{
		runner: indicatorScriptRunner[vo.KCandleVo]{
			executionTimeout: executionTimeout,
			inputTypeName:    "KCandle",
			inputTypes: map[string]reflect.Value{
				"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
			},
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
	return yaegiIndicatorScriptProxy.runner.execute(executionContext, script, resultType, kCandles, parameters)
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
	return yaegiIndicatorScriptProxy.runner.executeForEachElement(
		executionContext, script, resultType, kCandles, parameters)
}

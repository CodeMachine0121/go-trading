package script

import (
	"context"
	"reflect"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// YaegiContractIndicatorScriptProxy runs contract indicator scripts — scripts fed
// perpetual contract bars — with the same embedded interpreter, sandbox and allowance
// spot scripts run under.
//
// A contract script may name three types: the bar it is fed, the price line the bar
// carries three of, and the spot K candle the bar embeds, so that a helper written for
// indicator.KCandle can be handed the embedded candle as it is.
type YaegiContractIndicatorScriptProxy struct {
	runner indicatorScriptRunner[vo.ContractKCandleVo]
}

func NewYaegiContractIndicatorScriptProxy(executionTimeout time.Duration) *YaegiContractIndicatorScriptProxy {
	return &YaegiContractIndicatorScriptProxy{
		runner: indicatorScriptRunner[vo.ContractKCandleVo]{
			executionTimeout: executionTimeout,
			inputTypeName:    "ContractKCandle",
			inputTypes: map[string]reflect.Value{
				"ContractKCandle": reflect.ValueOf((*vo.ContractKCandleVo)(nil)),
				"PriceLine":       reflect.ValueOf((*vo.PriceLineVo)(nil)),
				"KCandle":         reflect.ValueOf((*vo.KCandleVo)(nil)),
			},
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
	return yaegiContractIndicatorScriptProxy.runner.execute(
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
	return yaegiContractIndicatorScriptProxy.runner.executeForEachElement(
		executionContext, script, resultType, contractKCandles, parameters)
}

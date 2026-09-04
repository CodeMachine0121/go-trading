package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// indicatorCalculationAssistantArguments is what the assistant sends to run one
// calculation. It may name a saved strategy or bring its own algorithm, never both.
type indicatorCalculationAssistantArguments struct {
	Symbol      string `json:"symbol"`
	Interval    string `json:"interval"`
	CandleCount int    `json:"candleCount"`
	StrategyID  uint   `json:"strategyId"`
	Script      string `json:"script"`
	ResultType  string `json:"resultType"`
	EndTime     string `json:"endTime"`
	// ParameterValues are what the named strategy's knobs are worth this time.
	// Anything left out keeps the value it was declared with.
	ParameterValues []strategyParameterValueAssistantArgument `json:"parameterValues"`
}

// strategyParameterValueAssistantArgument is what one knob is worth this run.
type strategyParameterValueAssistantArgument struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// IndicatorCalculationAssistantQuery lets the assistant run one indicator
// calculation, either from a saved strategy or from an algorithm it wrote itself.
//
// Naming a saved strategy is offered because that is how the question is actually
// asked — "look at BTCUSDT with my twenty-bar average" names a strategy, not a
// script. The alternative, making the assistant read the strategy and then send its
// algorithm back, costs an extra round trip and puts the whole script through the
// conversation twice for nothing.
//
// A strategy that is named wins over an algorithm that is sent, so that the two can
// never quietly disagree about which one ran.
type IndicatorCalculationAssistantQuery struct {
	indicatorCalculationApplication *IndicatorCalculationApplication
	strategyApplication             *StrategyApplication
}

func NewIndicatorCalculationAssistantQuery(
	indicatorCalculationApplication *IndicatorCalculationApplication,
	strategyApplication *StrategyApplication,
) *IndicatorCalculationAssistantQuery {
	return &IndicatorCalculationAssistantQuery{
		indicatorCalculationApplication: indicatorCalculationApplication,
		strategyApplication:             strategyApplication,
	}
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Name() string {
	return "calculate_indicator"
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Description() string {
	return "算一次指標。可以指名一支既有策略（strategyId），或自己帶一段算式（script）；" +
		"兩者都給時以 strategyId 為準。彙總刻度只接受 5m、15m、1h、4h、1d，未給視為 5m。" +
		"candleCount 是要餵幾根彙總 K 線，必須大於零。回傳的是指標值，不是 K 線。"
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"},` +
		`"interval":{"type":"string","enum":["5m","15m","1h","4h","1d"],"description":"彙總刻度"},` +
		`"candleCount":{"type":"integer","description":"要餵幾根彙總 K 線"},` +
		`"strategyId":{"type":"integer","description":"要用哪一支既有策略"},` +
		`"script":{"type":"string","description":"自帶的指標算式，未指名策略時使用"},` +
		`"resultType":{"type":"string","enum":["float","floatList","bool","boolList"],"description":"自帶算式的指標值種類"},` +
		`"endTime":{"type":"string","description":"算到哪個時間為止，RFC3339，未給視為現在"},` +
		`"parameterValues":{"type":"array","description":"這次每個參數是多少","items":{"type":"object","properties":{` +
		`"name":{"type":"string"},"value":{"type":"number"}},"required":["name","value"],"additionalProperties":false}}` +
		`},"required":["symbol","candleCount"],"additionalProperties":false}`
}

// Run works out one calculation and hands back its values.
//
// Every rule the calculation already obeys is obeyed here unrelaxed — an
// unrecognised coarseness, a count outside its bounds, an algorithm that will not
// run, too few candles to fill the count — and each comes back as the reason it was
// refused, which the assistant reads and may act on.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Run(
	executionContext context.Context, arguments string,
) (string, error) {
	calculationArguments := indicatorCalculationAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &calculationArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	endTime := time.Time{}
	if calculationArguments.EndTime != "" {
		namedEndTime, endTimeError := assistantMomentOf(calculationArguments.EndTime, "endTime")
		if endTimeError != nil {
			return "", endTimeError
		}

		endTime = namedEndTime
	}

	requestDto, requestError := indicatorCalculationAssistantQuery.requestFor(
		executionContext, calculationArguments, endTime)
	if requestError != nil {
		return "", requestError
	}

	resultDto, calculateError := indicatorCalculationAssistantQuery.indicatorCalculationApplication.CalculateIndicator(
		executionContext, requestDto)
	if calculateError != nil {
		return "", calculateError
	}

	payload, marshalError := json.Marshal(resultDto)
	if marshalError != nil {
		return "", fmt.Errorf("render indicator calculation: %w", marshalError)
	}

	return string(payload), nil
}

// requestFor is the calculation to run: the algorithm comes from the named strategy
// when one was named, and from the assistant itself when none was.
//
// Reading the strategy here rather than trusting what was sent is what keeps a named
// strategy honest — the algorithm and the value kind that run are the ones the
// strategy actually holds, whatever else arrived alongside the name.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) requestFor(
	executionContext context.Context,
	calculationArguments indicatorCalculationAssistantArguments,
	endTime time.Time,
) (dto.IndicatorCalculationRequestDto, error) {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(calculationArguments.ParameterValues))
	for _, parameterValue := range calculationArguments.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, dto.StrategyParameterValueDto{
			Name:  parameterValue.Name,
			Value: parameterValue.Value,
		})
	}

	requestDto := dto.IndicatorCalculationRequestDto{
		Symbol:              calculationArguments.Symbol,
		CandleCount:         calculationArguments.CandleCount,
		Script:              calculationArguments.Script,
		AggregationInterval: calculationArguments.Interval,
		ResultType:          calculationArguments.ResultType,
		EndTime:             endTime,
		Parameters:          make([]dto.StrategyParameterWriteDto, 0),
		ParameterValues:     parameterValueDtos,
	}

	if calculationArguments.StrategyID == 0 {
		return requestDto, nil
	}

	strategyDto, findError := indicatorCalculationAssistantQuery.strategyApplication.GetStrategy(
		executionContext, calculationArguments.StrategyID)
	if findError != nil {
		return dto.IndicatorCalculationRequestDto{}, findError
	}

	requestDto.Script = strategyDto.Script
	requestDto.ResultType = strategyDto.ResultType
	for _, parameterDto := range strategyDto.Parameters {
		requestDto.Parameters = append(requestDto.Parameters, dto.StrategyParameterWriteDto{
			Name:         parameterDto.Name,
			Kind:         parameterDto.Kind,
			DefaultValue: parameterDto.DefaultValue,
		})
	}

	return requestDto, nil
}

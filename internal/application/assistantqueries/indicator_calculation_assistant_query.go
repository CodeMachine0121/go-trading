package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// indicatorCalculationAssistantArguments is what the assistant sends to run one
// calculation. It may name a saved strategy or bring its own algorithm, never both.
type indicatorCalculationAssistantArguments struct {
	Symbol     string `json:"symbol"`
	Interval   string `json:"interval"`
	StrategyID uint   `json:"strategyId"`
	Script     string `json:"script"`
	ResultType string `json:"resultType"`
	// StartTime and EndTime are the stretch of market to read, RFC3339. How many
	// values come out of it depends on how much of it the symbol's market is open
	// for, so a stretch over a Taiwan night holds nothing at all.
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
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
//
// It no longer reads the strategy itself. Fetching the algorithm belongs to the use
// case that runs it — that is where the three gates are walked — so this capability
// hands over an identifier and never holds a script it did not write.
type IndicatorCalculationAssistantQuery struct {
	indicatorCalculationApplication *application.IndicatorCalculationApplication
}

func NewIndicatorCalculationAssistantQuery(
	indicatorCalculationApplication *application.IndicatorCalculationApplication,
) *IndicatorCalculationAssistantQuery {
	return &IndicatorCalculationAssistantQuery{
		indicatorCalculationApplication: indicatorCalculationApplication,
	}
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Name() string {
	return "calculate_indicator"
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Description() string {
	return "算一次指標。可以指名一支既有策略（strategyId），或自己帶一段算式（script）；" +
		"兩者都給時以 strategyId 為準。彙總刻度只接受 1m、5m、15m、1h、4h、1d，未給視為 1m。" +
		"startTime 與 endTime 是要看哪一段行情；會收盤的市場只數有交易的那些時間，" +
		"所以整段落在台股收盤時間裡的區間會被拒絕。回傳的是指標值，不是 K 線。" +
		"存下來的行情不夠長時不會被拒絕，而是用手上有的算：回傳的 requiredCandleCount 是填滿要幾根、" +
		"usedCandleCount 是實際用了幾根，兩者不同就表示這個讀數是以較少的行情算出來的，說結論時要講出來。" +
		"requiredCandleCount 已經含了回看根數，也已經扣掉收盤的時間，不要自己拿區間長度去推算它。"
}

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` +
		`"symbol":{"type":"string","description":"交易標的代號，例如 BTCUSDT"},` +
		`"interval":{"type":"string","enum":["1m","5m","15m","1h","4h","1d"],"description":"彙總刻度"},` +
		`"startTime":{"type":"string","description":"要看哪一段行情的起點，RFC3339"},` +
		`"strategyId":{"type":"integer","description":"要用哪一支既有策略"},` +
		`"script":{"type":"string","description":"自帶的指標算式，未指名策略時使用"},` +
		`"resultType":{"type":"string","enum":["float","floatList","bool","boolList"],"description":"自帶算式的指標值種類"},` +
		`"endTime":{"type":"string","description":"算到哪個時間為止，RFC3339，未給視為現在"},` +
		`"parameterValues":{"type":"array","description":"這次每個參數是多少","items":{"type":"object","properties":{` +
		`"name":{"type":"string"},"value":{"type":"number"}},"required":["name","value"],"additionalProperties":false}}` +
		`},"required":["symbol","startTime"],"additionalProperties":false}`
}

// Run works out one calculation and hands back its values.
//
// Every rule the calculation already obeys is obeyed here unrelaxed — an
// unrecognised coarseness, a count outside its bounds, an algorithm that will not
// run, a stretch of market too thin to yield a single value — and each comes back as
// the reason it was refused, which the assistant reads and may act on.
//
// A stretch merely shorter than the count asked for is not among them: the
// calculation answers over what is there, and the answer carries both the count a
// full one would have taken and the count it worked from. Those travel to the
// assistant as they are, so it can say a reading is based on less market than asked
// for instead of presenting it as complete.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) Run(
	executionContext context.Context, viewerID uint, arguments string,
) (string, error) {
	calculationArguments := indicatorCalculationAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &calculationArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	startTime, startTimeError := assistantMomentOf(calculationArguments.StartTime, "startTime")
	if startTimeError != nil {
		return "", startTimeError
	}

	endTime := time.Time{}
	if calculationArguments.EndTime != "" {
		namedEndTime, endTimeError := assistantMomentOf(calculationArguments.EndTime, "endTime")
		if endTimeError != nil {
			return "", endTimeError
		}

		endTime = namedEndTime
	}

	resultDto, calculateError := indicatorCalculationAssistantQuery.calculate(
		executionContext, viewerID, calculationArguments,
		indicatorCalculationAssistantQuery.requestFor(calculationArguments, startTime, endTime))
	if calculateError != nil {
		return "", calculateError
	}

	payload, marshalError := json.Marshal(resultDto)
	if marshalError != nil {
		return "", fmt.Errorf("render indicator calculation: %w", marshalError)
	}

	return string(payload), nil
}

// requestFor is everything about this calculation except the algorithm: where to
// read, how coarse, and what the knobs are worth this time.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) requestFor(
	calculationArguments indicatorCalculationAssistantArguments,
	startTime time.Time,
	endTime time.Time,
) dto.IndicatorCalculationRequestDto {
	parameterValueDtos := make([]dto.StrategyParameterValueDto, 0, len(calculationArguments.ParameterValues))
	for _, parameterValue := range calculationArguments.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, dto.StrategyParameterValueDto{
			Name:  parameterValue.Name,
			Value: parameterValue.Value,
		})
	}

	return dto.IndicatorCalculationRequestDto{
		Symbol:              calculationArguments.Symbol,
		StartTime:           startTime,
		AggregationInterval: calculationArguments.Interval,
		EndTime:             endTime,
		Parameters:          make([]dto.StrategyParameterWriteDto, 0),
		ParameterValues:     parameterValueDtos,
	}
}

// calculate runs the named strategy, or the algorithm the assistant wrote itself
// when it named none.
//
// The two paths are genuinely different and not a branch worth removing. A named
// strategy has an owner and may belong to somebody else, so it goes through the
// gates and the script is fetched on this side. An algorithm the assistant just
// composed belongs to the person who asked for it — there is nobody to hide it from
// — and no saved strategy to look up.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) calculate(
	executionContext context.Context,
	viewerID uint,
	calculationArguments indicatorCalculationAssistantArguments,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	if calculationArguments.StrategyID != 0 {
		return indicatorCalculationAssistantQuery.indicatorCalculationApplication.CalculateIndicator(
			executionContext, viewerID, calculationArguments.StrategyID, requestDto)
	}

	requestDto.Script = calculationArguments.Script
	requestDto.ResultType = calculationArguments.ResultType

	return indicatorCalculationAssistantQuery.indicatorCalculationApplication.CalculateAdHocIndicator(
		executionContext, requestDto)
}

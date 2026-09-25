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

// indicatorCalculationAssistantArguments names a saved strategy script or brings its own
// algorithm, never both.
type indicatorCalculationAssistantArguments struct {
	Symbol           string `json:"symbol"`
	Interval         string `json:"interval"`
	StrategyScriptID uint   `json:"strategyScriptId"`
	Script           string `json:"script"`
	ResultType       string `json:"resultType"`
	// StartTime and EndTime are RFC3339; only market-open time yields values, so a stretch over a
	// Taiwan night holds nothing.
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	// ParameterValues left out keep their declared values.
	ParameterValues []strategyScriptParameterValueAssistantArgument `json:"parameterValues"`
}

type strategyScriptParameterValueAssistantArgument struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// IndicatorCalculationAssistantQuery runs one indicator calculation from a saved strategy script
// (which wins over a sent algorithm) or from an algorithm the assistant wrote; fetching the script
// is left to the application so this never holds a script it did not write.
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
	return "算一次指標。可以指名一支既有策略腳本（strategyScriptId），或自己帶一段算式（script）；" +
		"兩者都給時以 strategyScriptId 為準。彙總刻度只接受 1m、5m、15m、1h、4h、1d，未給視為 1m。" +
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
		`"strategyScriptId":{"type":"integer","description":"要用哪一支既有策略腳本"},` +
		`"script":{"type":"string","description":"自帶的指標算式，未指名策略腳本時使用"},` +
		`"resultType":{"type":"string","enum":["float","floatList","bool","boolList","signal"],"description":"自帶算式的指標值種類"},` +
		`"endTime":{"type":"string","description":"算到哪個時間為止，RFC3339，未給視為現在"},` +
		`"parameterValues":{"type":"array","description":"這次每個參數是多少","items":{"type":"object","properties":{` +
		`"name":{"type":"string"},"value":{"type":"number"}},"required":["name","value"],"additionalProperties":false}}` +
		`},"required":["symbol","startTime"],"additionalProperties":false}`
}

// Run surfaces every refusal of the calculation as a readable reason; a stretch merely shorter
// than asked is answered, with required and used candle counts so the assistant can say so.
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

func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) requestFor(
	calculationArguments indicatorCalculationAssistantArguments,
	startTime time.Time,
	endTime time.Time,
) dto.IndicatorCalculationRequestDto {
	parameterValueDtos := make([]dto.StrategyScriptParameterValueDto, 0, len(calculationArguments.ParameterValues))
	for _, parameterValue := range calculationArguments.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, dto.StrategyScriptParameterValueDto{
			Name:  parameterValue.Name,
			Value: parameterValue.Value,
		})
	}

	return dto.IndicatorCalculationRequestDto{
		Symbol:              calculationArguments.Symbol,
		StartTime:           startTime,
		AggregationInterval: calculationArguments.Interval,
		EndTime:             endTime,
		Parameters:          make([]dto.StrategyScriptParameterWriteDto, 0),
		ParameterValues:     parameterValueDtos,
	}
}

// calculate lets the shared run-subject model decide between the named script and the inline
// algorithm, so the assistant cannot interpret "one or the other" differently.
func (indicatorCalculationAssistantQuery *IndicatorCalculationAssistantQuery) calculate(
	executionContext context.Context,
	viewerID uint,
	calculationArguments indicatorCalculationAssistantArguments,
	requestDto dto.IndicatorCalculationRequestDto,
) (dto.IndicatorCalculationResultDto, error) {
	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		calculationArguments.StrategyScriptID, calculationArguments.Script,
		calculationArguments.ResultType, nil)
	if subjectError != nil {
		return dto.IndicatorCalculationResultDto{}, subjectError
	}

	return indicatorCalculationAssistantQuery.indicatorCalculationApplication.CalculateIndicator(
		executionContext, viewerID, runSubjectDomain, requestDto)
}

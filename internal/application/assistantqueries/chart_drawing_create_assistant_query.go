package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingCreateAssistantQuery lets the assistant draw on the asker's chart; drawings touch no money, so they take effect at once instead of waiting for confirmation.
type ChartDrawingCreateAssistantQuery struct {
	chartDrawingApplication *application.ChartDrawingApplication
	limits                  vo.ChartDrawingLimitsVo
}

func NewChartDrawingCreateAssistantQuery(
	chartDrawingApplication *application.ChartDrawingApplication, limits vo.ChartDrawingLimitsVo,
) *ChartDrawingCreateAssistantQuery {
	return &ChartDrawingCreateAssistantQuery{chartDrawingApplication: chartDrawingApplication, limits: limits}
}

func (chartDrawingCreateAssistantQuery *ChartDrawingCreateAssistantQuery) Name() string {
	return "create_chart_drawings"
}

func (chartDrawingCreateAssistantQuery *ChartDrawingCreateAssistantQuery) Description() string {
	return fmt.Sprintf("在使用者的 K 線圖上畫一批線（一次 1 到 %d 個，整批成功或整批不畫，一份圖最多 %d 個、錨點加總最多 %d 個）。"+
		"interval 要說出你是看哪一種彙總刻度的 K 線畫的；每個錨點的 openTime 必須正好是那個刻度上一根真的 K 線的起始時間（RFC3339），"+
		"先用 get_k_candle_series 找出高低點再畫。每個錨點的價格二選一：price 直接給價格，或 snapTo 吸附那一根的 open／high／low／close"+
		"（畫支撐壓力時優先吸附，不要自己編價位）。趨勢線、射線、斐波那契回撤、矩形、日期與價格範圍要 2 個錨點；水平線、水平射線、垂直線要 1 個。"+
		"奇異筆（brush）只能由使用者手畫，你不能畫。"+
		"樣式可以不給。你畫的線都標成助手畫的，之後可以用 remove_chart_drawings 移除；使用者手畫的你動不到。",
		chartDrawingCreateAssistantQuery.limits.AssistantBatchCapacity, chartDrawingCreateAssistantQuery.limits.BoardCapacity,
		chartDrawingCreateAssistantQuery.limits.BoardAnchorCapacity)
}

func (chartDrawingCreateAssistantQuery *ChartDrawingCreateAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + chartDrawingPlaceArgumentSchema + `,` +
		`"interval":{"type":"string","enum":["1m","5m","15m","1h","4h","1d"],"description":"錨點對齊的彙總刻度"},` +
		`"drawings":{"type":"array","items":{"type":"object","properties":{` +
		`"kind":{"type":"string","enum":["trendLine","ray","horizontalLine","horizontalRay","verticalLine","fibonacciRetracement","rectangle","dateAndPriceRange"]},` +
		`"anchors":{"type":"array","items":{"type":"object","properties":{` +
		`"openTime":{"type":"string","description":"那一根 K 線的起始時間，RFC3339"},` +
		`"price":{"type":"string","description":"價格，大於零；與 snapTo 恰好給一個"},` +
		`"snapTo":{"type":"string","enum":["open","high","low","close"],"description":"吸附到那一根的哪個價格；與 price 恰好給一個"}` +
		`},"required":["openTime"],"additionalProperties":false}},` +
		`"color":{"type":"string","enum":["red","orange","yellow","green","teal","cyan","blue","indigo","purple","pink","gray","silver"]},` +
		`"lineWidth":{"type":"integer","minimum":1,"maximum":4},` +
		`"lineStyle":{"type":"string","enum":["solid","dashed","dotted"]}` +
		`},"required":["kind","anchors"],"additionalProperties":false}}` +
		`},"required":["market","symbol","interval","drawings"],"additionalProperties":false}`
}

func (chartDrawingCreateAssistantQuery *ChartDrawingCreateAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	createArguments := chartDrawingCreateAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &createArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	additionDto, addError := chartDrawingCreateAssistantQuery.chartDrawingApplication.AddAssistantDrawings(
		executionContext, createArguments.ToBatchDto(origin.ViewerID, origin.ConversationID, origin.TurnID))
	if addError != nil {
		return "", addError
	}

	payload, marshalError := json.Marshal(additionDto)
	if marshalError != nil {
		return "", fmt.Errorf("render chart drawings: %w", marshalError)
	}

	return string(payload), nil
}

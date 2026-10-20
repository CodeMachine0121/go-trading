package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingListAssistantQuery shows the whole board with each drawing's source, so the assistant knows which lines are its own.
type ChartDrawingListAssistantQuery struct {
	chartDrawingApplication *application.ChartDrawingApplication
}

func NewChartDrawingListAssistantQuery(
	chartDrawingApplication *application.ChartDrawingApplication,
) *ChartDrawingListAssistantQuery {
	return &ChartDrawingListAssistantQuery{chartDrawingApplication: chartDrawingApplication}
}

func (chartDrawingListAssistantQuery *ChartDrawingListAssistantQuery) Name() string {
	return "list_chart_drawings"
}

func (chartDrawingListAssistantQuery *ChartDrawingListAssistantQuery) Description() string {
	return "列出使用者某一張 K 線圖（現貨或合約的某個交易標的）上的每一個繪圖：編號、種類、錨點、樣式與來源" +
		"（source 為 manual 是使用者手畫的、assistant 是助手畫的）。畫之前先看一下，不要重複畫。"
}

func (chartDrawingListAssistantQuery *ChartDrawingListAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + chartDrawingPlaceArgumentSchema +
		`},"required":["market","symbol"],"additionalProperties":false}`
}

func (chartDrawingListAssistantQuery *ChartDrawingListAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	placeArguments := chartDrawingPlaceAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &placeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	boardDto, getError := chartDrawingListAssistantQuery.chartDrawingApplication.GetBoard(
		executionContext, placeArguments.ToPlaceDto(origin.ViewerID))
	if getError != nil {
		return "", getError
	}

	payload, marshalError := json.Marshal(boardDto)
	if marshalError != nil {
		return "", fmt.Errorf("render chart drawing board: %w", marshalError)
	}

	return string(payload), nil
}

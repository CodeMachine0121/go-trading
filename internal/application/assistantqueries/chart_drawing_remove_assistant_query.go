package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingRemoveAssistantQuery removes only what an assistant drew; naming a hand drawing removes nothing at all.
type ChartDrawingRemoveAssistantQuery struct {
	chartDrawingApplication *application.ChartDrawingApplication
}

func NewChartDrawingRemoveAssistantQuery(
	chartDrawingApplication *application.ChartDrawingApplication,
) *ChartDrawingRemoveAssistantQuery {
	return &ChartDrawingRemoveAssistantQuery{chartDrawingApplication: chartDrawingApplication}
}

func (chartDrawingRemoveAssistantQuery *ChartDrawingRemoveAssistantQuery) Name() string {
	return "remove_chart_drawings"
}

func (chartDrawingRemoveAssistantQuery *ChartDrawingRemoveAssistantQuery) Description() string {
	return "移除 K 線圖上助手畫的繪圖：不給 drawingIds 就移除助手畫的全部，給了就只移除那幾個。" +
		"只能移除助手畫的——指名裡只要有一個是使用者手畫的，整次被拒絕、一個都不刪。要改一條線就移除再畫。"
}

func (chartDrawingRemoveAssistantQuery *ChartDrawingRemoveAssistantQuery) ArgumentSchema() string {
	return `{"type":"object","properties":{` + chartDrawingPlaceArgumentSchema + `,` +
		`"drawingIds":{"type":"array","items":{"type":"integer"},"description":"要移除的繪圖編號；不給即移除助手畫的全部"}` +
		`},"required":["market","symbol"],"additionalProperties":false}`
}

func (chartDrawingRemoveAssistantQuery *ChartDrawingRemoveAssistantQuery) Run(
	executionContext context.Context, origin vo.AssistantQueryOriginVo, arguments string,
) (string, error) {
	removeArguments := chartDrawingRemoveAssistantArguments{}
	if unmarshalError := json.Unmarshal([]byte(arguments), &removeArguments); unmarshalError != nil {
		return "", fmt.Errorf("%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}

	resultDto, removeError := chartDrawingRemoveAssistantQuery.chartDrawingApplication.RemoveAssistantDrawings(
		executionContext, dto.AssistantChartDrawingRemovalDto{
			Place:      removeArguments.ToPlaceDto(origin.ViewerID),
			DrawingIDs: removeArguments.DrawingIDs,
		})
	if removeError != nil {
		return "", removeError
	}

	payload, marshalError := json.Marshal(resultDto)
	if marshalError != nil {
		return "", fmt.Errorf("render removed chart drawings: %w", marshalError)
	}

	return string(payload), nil
}

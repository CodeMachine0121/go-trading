package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type ChartDrawingApplication struct {
	chartDrawingService *service.ChartDrawingService
}

func NewChartDrawingApplication(chartDrawingService *service.ChartDrawingService) *ChartDrawingApplication {
	return &ChartDrawingApplication{chartDrawingService: chartDrawingService}
}

func (chartDrawingApplication *ChartDrawingApplication) GetBoard(
	executionContext context.Context, placeDto dto.ChartDrawingBoardPlaceDto,
) (dto.ChartDrawingBoardDto, error) {
	return chartDrawingApplication.chartDrawingService.GetBoard(executionContext, placeDto)
}

func (chartDrawingApplication *ChartDrawingApplication) ReplaceBoard(
	executionContext context.Context, replaceDto dto.ChartDrawingBoardReplaceDto,
) (dto.ChartDrawingBoardDto, error) {
	return chartDrawingApplication.chartDrawingService.ReplaceBoard(executionContext, replaceDto)
}

func (chartDrawingApplication *ChartDrawingApplication) AddAssistantDrawings(
	executionContext context.Context, batchDto dto.AssistantChartDrawingBatchDto,
) (dto.AssistantChartDrawingAdditionDto, error) {
	return chartDrawingApplication.chartDrawingService.AddAssistantDrawings(executionContext, batchDto)
}

func (chartDrawingApplication *ChartDrawingApplication) RemoveAssistantDrawings(
	executionContext context.Context, removalDto dto.AssistantChartDrawingRemovalDto,
) (dto.AssistantChartDrawingRemovalResultDto, error) {
	return chartDrawingApplication.chartDrawingService.RemoveAssistantDrawings(executionContext, removalDto)
}

package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type ChartDrawingController struct {
	chartDrawingApplication *application.ChartDrawingApplication
}

func NewChartDrawingController(chartDrawingApplication *application.ChartDrawingApplication) *ChartDrawingController {
	return &ChartDrawingController{chartDrawingApplication: chartDrawingApplication}
}

func (chartDrawingController *ChartDrawingController) GetBoard(ginContext *gin.Context) {
	boardDto, err := chartDrawingController.chartDrawingApplication.GetBoard(
		ginContext.Request.Context(), chartDrawingController.placeOf(ginContext))
	if err != nil {
		chartDrawingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, boardDto)
}

func (chartDrawingController *ChartDrawingController) ReplaceBoard(ginContext *gin.Context) {
	var boardRequest models.ChartDrawingBoardRequest
	if bindError := ginContext.ShouldBindJSON(&boardRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": "要整份交回繪圖板，並以 revision 說出是根據第幾版改的（從沒存過給 0）：" + bindError.Error(),
		})
		return
	}

	boardDto, err := chartDrawingController.chartDrawingApplication.ReplaceBoard(
		ginContext.Request.Context(), boardRequest.ToReplaceDto(chartDrawingController.placeOf(ginContext)))
	if err != nil {
		chartDrawingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, boardDto)
}

func (chartDrawingController *ChartDrawingController) AddAssistantDrawings(ginContext *gin.Context) {
	var batchRequest models.AssistantChartDrawingBatchRequest
	if bindError := ginContext.ShouldBindJSON(&batchRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	additionDto, err := chartDrawingController.chartDrawingApplication.AddAssistantDrawings(
		ginContext.Request.Context(), batchRequest.ToBatchDto(chartDrawingController.placeOf(ginContext)))
	if err != nil {
		chartDrawingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, additionDto)
}

// RemoveAssistantDrawings removes every assistant drawing when ids is absent or empty.
func (chartDrawingController *ChartDrawingController) RemoveAssistantDrawings(ginContext *gin.Context) {
	drawingIDs := make([]uint, 0)
	for _, token := range strings.Split(ginContext.Query("ids"), ",") {
		trimmedToken := strings.TrimSpace(token)
		if trimmedToken == "" {
			continue
		}

		drawingID, parseError := strconv.ParseUint(trimmedToken, 10, strconv.IntSize)
		if parseError != nil || drawingID == 0 || drawingID > math.MaxInt64 {
			ginContext.JSON(http.StatusBadRequest, gin.H{"message": "ids 必須是以逗號分隔的正整數繪圖編號"})
			return
		}
		drawingIDs = append(drawingIDs, uint(drawingID))
	}

	resultDto, err := chartDrawingController.chartDrawingApplication.RemoveAssistantDrawings(
		ginContext.Request.Context(), dto.AssistantChartDrawingRemovalDto{
			Place: chartDrawingController.placeOf(ginContext), DrawingIDs: drawingIDs,
		})
	if err != nil {
		chartDrawingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

func (chartDrawingController *ChartDrawingController) placeOf(ginContext *gin.Context) dto.ChartDrawingBoardPlaceDto {
	return dto.ChartDrawingBoardPlaceDto{
		OwnerID: middlewares.CurrentUserID(ginContext),
		Market:  ginContext.Param("market"),
		Symbol:  ginContext.Param("symbol"),
	}
}

func (chartDrawingController *ChartDrawingController) respondWithError(ginContext *gin.Context, err error) {
	if revisionConflictError, isRevisionConflict := errors.AsType[domains.ChartDrawingBoardRevisionConflictError](err); isRevisionConflict {
		ginContext.JSON(http.StatusConflict, gin.H{
			"message": revisionConflictError.Error(), "board": revisionConflictError.CurrentBoard,
		})
		return
	}
	if errors.Is(err, domains.ErrChartDrawingValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrChartDrawingNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrChartDrawingHandDrawn) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type ContractPositionStatisticController struct {
	positionStatisticApplication *application.ContractPositionStatisticApplication
}

func NewContractPositionStatisticController(
	positionStatisticApplication *application.ContractPositionStatisticApplication,
) *ContractPositionStatisticController {
	return &ContractPositionStatisticController{positionStatisticApplication: positionStatisticApplication}
}

// GetStatisticsInRange handles GET /contract-position-statistics.
func (statisticController *ContractPositionStatisticController) GetStatisticsInRange(
	ginContext *gin.Context,
) {
	startTime, startTimeError := time.Parse(time.RFC3339, ginContext.Query("startTime"))
	if startTimeError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "startTime 必須為 RFC3339 格式的時間"})

		return
	}

	endTime, endTimeError := time.Parse(time.RFC3339, ginContext.Query("endTime"))
	if endTimeError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "endTime 必須為 RFC3339 格式的時間"})

		return
	}

	statisticDtos, findError := statisticController.positionStatisticApplication.
		GetStatisticsInRange(ginContext.Request.Context(), dto.KCandleQueryDto{
			Symbol:    ginContext.Query("symbol"),
			StartTime: startTime,
			EndTime:   endTime,
		})
	if errors.Is(findError, domains.ErrContractPositionStatisticValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": findError.Error()})

		return
	}
	if findError != nil {
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": findError.Error()})

		return
	}

	ginContext.JSON(http.StatusOK, statisticDtos)
}

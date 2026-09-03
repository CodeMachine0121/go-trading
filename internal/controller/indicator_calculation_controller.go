package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// IndicatorCalculationController exposes the indicator calculation use case over HTTP.
type IndicatorCalculationController struct {
	indicatorCalculationApplication *application.IndicatorCalculationApplication
}

func NewIndicatorCalculationController(
	indicatorCalculationApplication *application.IndicatorCalculationApplication,
) *IndicatorCalculationController {
	return &IndicatorCalculationController{
		indicatorCalculationApplication: indicatorCalculationApplication,
	}
}

// CalculateIndicator handles POST /indicator-calculations.
func (indicatorCalculationController *IndicatorCalculationController) CalculateIndicator(
	ginContext *gin.Context,
) {
	var indicatorCalculationRequest models.IndicatorCalculationRequest

	if bindError := ginContext.ShouldBindJSON(&indicatorCalculationRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	resultDto, err := indicatorCalculationController.indicatorCalculationApplication.CalculateIndicator(ginContext.Request.Context(),
		indicatorCalculationRequest.ToRequestDto())
	if err != nil {
		indicatorCalculationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// respondWithError separates "your request was wrong" from "your script cannot run",
// so a caller can tell the two apart without reading the message.
func (indicatorCalculationController *IndicatorCalculationController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrIndicatorCalculationValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptFailed) {
		ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

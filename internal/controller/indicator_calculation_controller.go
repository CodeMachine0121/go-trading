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
	context *gin.Context,
) {
	var indicatorCalculationRequest models.IndicatorCalculationRequest

	if bindError := context.ShouldBindJSON(&indicatorCalculationRequest); bindError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	resultDto, err := indicatorCalculationController.indicatorCalculationApplication.CalculateIndicator(
		indicatorCalculationRequest.ToRequestDto())
	if err != nil {
		indicatorCalculationController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, resultDto)
}

// respondWithError separates "your request was wrong" from "your script cannot run",
// so a caller can tell the two apart without reading the message.
func (indicatorCalculationController *IndicatorCalculationController) respondWithError(
	context *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrIndicatorCalculationValidation) {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptFailed) {
		context.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	context.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

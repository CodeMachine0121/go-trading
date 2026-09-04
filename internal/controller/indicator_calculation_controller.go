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

// respondWithError separates what went wrong by what the caller has to go and change,
// so the answer can be told apart without reading the message: the request itself, a
// knob's name, the script, or this system.
func (indicatorCalculationController *IndicatorCalculationController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrIndicatorCalculationValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// A knob the script reached for that nobody declared is the caller's mistake, not
	// the script's and not this system's. Left to fall through it would be answered
	// as a gateway failure — telling somebody the backend broke when what happened is
	// that they renamed a knob and forgot the line that reads it.
	if parameterName, isUndeclared := domains.UndeclaredParameterName(err); isUndeclared {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
			// The name travels as a value, not only inside the sentence: a caller
			// telling this failure apart by reading prose would be matching on words
			// written for a person, which change whenever the wording improves.
			"parameterName": parameterName,
		})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptFailed) {
		ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

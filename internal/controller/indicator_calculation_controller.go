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
// stretch of market too thin to answer, a knob's name, the script, or this system.
//
// The order matters. Each of the specific failures is also a validation failure, so
// every one of them has to be asked about before the general validation answer —
// otherwise it is caught there first and arrives stripped of the values that make it
// actionable.
func (indicatorCalculationController *IndicatorCalculationController) respondWithError(
	ginContext *gin.Context, err error,
) {
	// Naming the input at fault is what lets a caller put the sentence where the
	// person can act on it. Only this one validation failure has a specific input to
	// name — the rest are answered as they were.
	if errors.Is(err, domains.ErrIndicatorCalculationCandleCountExceeded) {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
			"field":   "candleCount",
		})
		return
	}
	// A stretch too thin to yield even one value is the other failure with a specific
	// way out, and it is the *opposite* way out from the one above: read the market
	// more finely, or fill in the missing history. Both counts travel as values, not
	// only inside the sentence, so a caller can put them where the person can act on
	// them. Left to fall through to the general validation answer below, it would
	// arrive as a sentence with no numbers a caller could use.
	if availableCandleCount, minimumCandleCount, isTooThin := domains.CandleCoverageShortfall(
		err); isTooThin {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":              err.Error(),
			"availableCandleCount": availableCandleCount,
			"minimumCandleCount":   minimumCandleCount,
		})
		return
	}
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

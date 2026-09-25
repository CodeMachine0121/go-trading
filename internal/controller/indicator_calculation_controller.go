package controller

import (
	"context"
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

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
	indicatorCalculationController.calculate(ginContext, indicatorCalculationController.indicatorCalculationApplication.CalculateIndicator)
}

// CalculateContractIndicator handles POST /contract-indicator-calculations with the same body and answers as the spot route, over contract bars.
func (indicatorCalculationController *IndicatorCalculationController) CalculateContractIndicator(
	ginContext *gin.Context,
) {
	indicatorCalculationController.calculate(
		ginContext, indicatorCalculationController.indicatorCalculationApplication.CalculateContractIndicator)
}

func (indicatorCalculationController *IndicatorCalculationController) calculate(
	ginContext *gin.Context,
	runCalculation func(
		executionContext context.Context,
		viewerID uint,
		runSubjectDomain domains.RunSubjectDomain,
		requestDto dto.IndicatorCalculationRequestDto,
	) (dto.IndicatorCalculationResultDto, error),
) {
	var indicatorCalculationRequest models.IndicatorCalculationRequest

	if bindError := ginContext.ShouldBindJSON(&indicatorCalculationRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		indicatorCalculationRequest.StrategyScriptID,
		indicatorCalculationRequest.Script,
		indicatorCalculationRequest.ResultType,
		indicatorCalculationRequest.ToParameterWriteDtos())
	if subjectError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": subjectError.Error()})
		return
	}

	resultDto, err := runCalculation(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runSubjectDomain,
		indicatorCalculationRequest.ToRequestDto())
	if err != nil {
		indicatorCalculationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// respondWithError must check each specific failure before the general validation one, since they all wrap it and would otherwise lose their actionable values.
func (indicatorCalculationController *IndicatorCalculationController) respondWithError(
	ginContext *gin.Context, err error,
) {
	// Missing, foreign and unpublished scripts all answer 404 so callers cannot probe which identifiers exist.
	if errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyScriptMarketDataKindMismatch) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrIndicatorCalculationCandleCountExceeded) {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
			// The stretch is blamed because shortening it (or a coarser interval) is the way out.
			"field": "startTime",
		})
		return
	}
	// No finer or coarser interval puts trading into a closed market, so this is flagged as a value to send the person to pick an open time.
	if errors.Is(err, domains.ErrObservationWindowHoldsNoTrading) {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":                         err.Error(),
			"observationWindowHoldsNoTrading": true,
		})
		return
	}
	// The opposite remedy from the above (finer interval or more history); both counts travel as values so callers can act on them.
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
	// An undeclared parameter is the caller's mistake; falling through would misreport it as a gateway failure.
	if parameterName, isUndeclared := domains.UndeclaredParameterName(err); isUndeclared {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":       err.Error(),
			"parameterName": parameterName,
		})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptCompartmentsBusy) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{
			"message":          err.Error(),
			"compartmentsBusy": true,
		})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptFailed) {
		ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

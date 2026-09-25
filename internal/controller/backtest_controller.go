package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

type BacktestController struct {
	backtestApplication *application.BacktestApplication
}

func NewBacktestController(backtestApplication *application.BacktestApplication) *BacktestController {
	return &BacktestController{backtestApplication: backtestApplication}
}

// RunBacktest handles POST /backtests.
func (backtestController *BacktestController) RunBacktest(ginContext *gin.Context) {
	var backtestRequest models.BacktestRequest

	if bindError := ginContext.ShouldBindJSON(&backtestRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		backtestRequest.StrategyScriptID, backtestRequest.Script, "", backtestRequest.ToParameterWriteDtos())
	if subjectError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": subjectError.Error()})
		return
	}

	resultDto, err := backtestController.backtestApplication.RunBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runSubjectDomain,
		backtestRequest.ToRequestDto())
	if err != nil {
		backtestController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// RunContractBacktest replays a contract strategy script and returns the report without storing anything.
func (backtestController *BacktestController) RunContractBacktest(ginContext *gin.Context) {
	var contractBacktestRequest models.ContractBacktestRequest

	if bindError := ginContext.ShouldBindJSON(&contractBacktestRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(
		contractBacktestRequest.StrategyScriptID, contractBacktestRequest.Script, "",
		contractBacktestRequest.ToParameterWriteDtos())
	if subjectError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": subjectError.Error()})
		return
	}

	resultDto, err := backtestController.backtestApplication.RunContractBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runSubjectDomain,
		contractBacktestRequest.ToRequestDto())
	if err != nil {
		backtestController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// respondWithError answers script, parameter and system failures exactly as indicator calculation does, so one broken script fails the same way on both routes.
func (backtestController *BacktestController) respondWithError(ginContext *gin.Context, err error) {
	// Missing, foreign and unpublished scripts all answer 404 so callers cannot probe which identifiers exist.
	if errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if fieldName, namesField := domains.BacktestFieldName(err); namesField {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
			"field":   fieldName,
		})
		return
	}
	if errors.Is(err, domains.ErrBacktestValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyScriptMarketDataKindMismatch) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// The parameter name travels as a value so callers need not parse the message.
	if parameterName, isUndeclared := domains.UndeclaredParameterName(err); isUndeclared {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":       err.Error(),
			"parameterName": parameterName,
		})
		return
	}
	// Answered apart from a script failure that shares the status: this one is fixed by asking for less.
	if errors.Is(err, domains.ErrBacktestTimeAllowanceSpent) {
		ginContext.JSON(http.StatusUnprocessableEntity, gin.H{
			"message":            err.Error(),
			"timeAllowanceSpent": true,
		})
		return
	}
	if errors.Is(err, domains.ErrIndicatorScriptFailed) {
		ginContext.JSON(http.StatusUnprocessableEntity, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

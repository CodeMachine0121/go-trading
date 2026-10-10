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

// SubmitBacktest handles POST /backtests: the request is judged now, the replay runs later, and the run is answered at once.
func (backtestController *BacktestController) SubmitBacktest(ginContext *gin.Context) {
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

	runDto, err := backtestController.backtestApplication.SubmitBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runSubjectDomain,
		backtestRequest.ToRequestDto())
	if err != nil {
		backtestController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, runDto)
}

// SubmitContractBacktest queues a contract strategy script replay and answers the run at once.
func (backtestController *BacktestController) SubmitContractBacktest(ginContext *gin.Context) {
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

	runDto, err := backtestController.backtestApplication.SubmitContractBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runSubjectDomain,
		contractBacktestRequest.ToRequestDto())
	if err != nil {
		backtestController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, runDto)
}

// respondWithError answers what can be refused before a run is queued; script failures surface later on the run itself.
func (backtestController *BacktestController) respondWithError(ginContext *gin.Context, err error) {
	// Missing, foreign and unpublished scripts all answer 404 so callers cannot probe which identifiers exist.
	if errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if ownerActiveLimit, limitReached := domains.BacktestRunOwnerLimit(err); limitReached {
		ginContext.JSON(http.StatusTooManyRequests, gin.H{
			"message":          err.Error(),
			"ownerActiveLimit": ownerActiveLimit,
		})
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

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

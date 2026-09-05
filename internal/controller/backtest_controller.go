package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// BacktestController exposes the strategy backtest use case over HTTP.
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

	resultDto, err := backtestController.backtestApplication.RunBacktest(
		ginContext.Request.Context(), backtestRequest.ToRequestDto())
	if err != nil {
		backtestController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// respondWithError separates what went wrong by what the caller has to go and change:
// the conditions of the replay, a knob's name, the script, or this system.
//
// The last three are answered exactly as a plain indicator calculation answers them —
// deliberately, because the same script fails the same way whichever of the two asked
// it to run, and a screen showing both should not have to tell two stories about one
// broken line.
func (backtestController *BacktestController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrBacktestValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// A knob the script reached for that nobody declared is the caller's mistake, not
	// the script's and not this system's. The name travels as a value, not only inside
	// the sentence: a caller telling this failure apart by reading prose would be
	// matching on words written for a person, which change whenever the wording does.
	if parameterName, isUndeclared := domains.UndeclaredParameterName(err); isUndeclared {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":       err.Error(),
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

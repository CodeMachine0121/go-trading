package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// TradingStrategyBacktestController exposes replaying a trading strategy over HTTP.
type TradingStrategyBacktestController struct {
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication
}

func NewTradingStrategyBacktestController(
	tradingStrategyBacktestApplication *application.TradingStrategyBacktestApplication,
) *TradingStrategyBacktestController {
	return &TradingStrategyBacktestController{
		tradingStrategyBacktestApplication: tradingStrategyBacktestApplication,
	}
}

// RunTradingStrategyBacktest handles POST /trading-strategies/:id/backtests.
//
// A replay hangs off the trading strategy it is about, for the reason a bot's rounds
// hang off the bot: it is something done *to* that one, not a thing of its own.
func (controller *TradingStrategyBacktestController) RunTradingStrategyBacktest(
	ginContext *gin.Context,
) {
	tradingStrategyID, idIsReadable := controller.readID(ginContext)
	if !idIsReadable {
		return
	}

	var request models.TradingStrategyBacktestRequest

	if bindError := ginContext.ShouldBindJSON(&request); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	resultDto, err := controller.tradingStrategyBacktestApplication.RunTradingStrategyBacktest(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyID,
		request.ToRequestDto())
	if err != nil {
		controller.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

// RunContractTradingStrategyBacktest replays one of this person's contract trading
// strategies on a contract account, by its own trading mode.
func (controller *TradingStrategyBacktestController) RunContractTradingStrategyBacktest(
	ginContext *gin.Context,
) {
	tradingStrategyID, idIsReadable := controller.readID(ginContext)
	if !idIsReadable {
		return
	}

	var request models.ContractTradingStrategyBacktestRequest

	if bindError := ginContext.ShouldBindJSON(&request); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	resultDto, err := controller.tradingStrategyBacktestApplication.RunContractTradingStrategyBacktest(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyID,
		request.ToRequestDto())
	if err != nil {
		controller.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, resultDto)
}

func (controller *TradingStrategyBacktestController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "交易策略識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError separates what went wrong by what the caller has to go and change.
//
// It answers exactly as replaying a single strategy script answers — deliberately.
// The same broken line fails the same way whichever kind of replay asked it to run,
// and a screen showing both should not have to tell two stories about one of them.
func (controller *TradingStrategyBacktestController) respondWithError(
	ginContext *gin.Context, err error,
) {
	// A trading strategy that is not there and one belonging to somebody else arrive
	// as one refusal and leave as one 404; so does a strategy script a source names
	// and the caller may not read.
	if errors.Is(err, domains.ErrTradingStrategyNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	// Naming the input at fault is what lets a caller put the sentence where the
	// person can act on it. The name travels as a value, not inside the sentence.
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
	if parameterName, isUndeclared := domains.UndeclaredParameterName(err); isUndeclared {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message":       err.Error(),
			"parameterName": parameterName,
		})
		return
	}
	// Nothing was wrong with what was asked; it could not be answered in time.
	// Said apart from a script failure, which shares the status: the one is fixed by
	// asking for less, the other by fixing the script.
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

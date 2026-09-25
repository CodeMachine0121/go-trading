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

// RunContractTradingStrategyBacktest replays a contract trading strategy using the strategy's own trading mode.
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

// respondWithError answers exactly as a single strategy script replay does, so the same broken script fails the same way on both routes.
func (controller *TradingStrategyBacktestController) respondWithError(
	ginContext *gin.Context, err error,
) {
	// Missing and foreign strategies, and unreadable scripts named by a source, all answer the same 404.
	if errors.Is(err, domains.ErrTradingStrategyNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	// The field name travels as a value so callers need not parse the message.
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

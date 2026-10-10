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

// SubmitTradingStrategyBacktest handles POST /trading-strategies/:id/backtests: judged now, replayed later, the run answered at once.
func (controller *TradingStrategyBacktestController) SubmitTradingStrategyBacktest(
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

	runDto, err := controller.tradingStrategyBacktestApplication.SubmitTradingStrategyBacktest(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyID,
		request.ToRequestDto())
	if err != nil {
		controller.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, runDto)
}

// SubmitContractTradingStrategyBacktest queues a contract trading strategy replay under the strategy's own trading mode.
func (controller *TradingStrategyBacktestController) SubmitContractTradingStrategyBacktest(
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

	runDto, err := controller.tradingStrategyBacktestApplication.SubmitContractTradingStrategyBacktest(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyID,
		request.ToRequestDto())
	if err != nil {
		controller.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, runDto)
}

func (controller *TradingStrategyBacktestController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "交易策略識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError answers exactly as a single strategy script submit does, so the same refusal reads the same on both routes.
func (controller *TradingStrategyBacktestController) respondWithError(
	ginContext *gin.Context, err error,
) {
	// Missing and foreign strategies, and unreadable scripts named by a source, all answer the same 404.
	if errors.Is(err, domains.ErrTradingStrategyNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) {
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
	// The field name travels as a value so callers need not parse the message.
	if fieldName, namesField := domains.BacktestFieldName(err); namesField {
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"message": err.Error(),
			"field":   fieldName,
		})
		return
	}
	if errors.Is(err, domains.ErrBacktestValidation) ||
		errors.Is(err, domains.ErrStrategyScriptNotYours) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

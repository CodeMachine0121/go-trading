package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type BacktestRunController struct {
	backtestRunApplication *application.BacktestRunApplication
}

func NewBacktestRunController(backtestRunApplication *application.BacktestRunApplication) *BacktestRunController {
	return &BacktestRunController{backtestRunApplication: backtestRunApplication}
}

// ListRuns handles GET /backtest-runs; limit and the two source filters are read leniently, falling back rather than failing.
func (backtestRunController *BacktestRunController) ListRuns(ginContext *gin.Context) {
	limit, _ := strconv.Atoi(ginContext.Query("limit"))
	strategyScriptID, _ := strconv.ParseUint(ginContext.Query("strategyScriptId"), 10, strconv.IntSize)
	tradingStrategyID, _ := strconv.ParseUint(ginContext.Query("tradingStrategyId"), 10, strconv.IntSize)

	pageDto, err := backtestRunController.backtestRunApplication.ListRuns(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), dto.BacktestRunListQueryDto{
			Status:            ginContext.Query("status"),
			StrategyScriptID:  uint(strategyScriptID),
			TradingStrategyID: uint(tradingStrategyID),
			Limit:             limit,
		})
	if err != nil {
		backtestRunController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, pageDto)
}

// GetRun handles GET /backtest-runs/:id; waitSeconds keeps the answer back until the run ends or the wait is spent.
func (backtestRunController *BacktestRunController) GetRun(ginContext *gin.Context) {
	id, idIsReadable := backtestRunController.readID(ginContext)
	if !idIsReadable {
		return
	}
	waitSeconds, _ := strconv.Atoi(ginContext.Query("waitSeconds"))

	detailDto, err := backtestRunController.backtestRunApplication.GetRun(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id,
		time.Duration(waitSeconds)*time.Second)
	if err != nil {
		backtestRunController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, detailDto)
}

// CancelRun handles POST /backtest-runs/:id/cancel.
func (backtestRunController *BacktestRunController) CancelRun(ginContext *gin.Context) {
	id, idIsReadable := backtestRunController.readID(ginContext)
	if !idIsReadable {
		return
	}

	runDto, err := backtestRunController.backtestRunApplication.CancelRun(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		backtestRunController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, runDto)
}

func (backtestRunController *BacktestRunController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "回測輪次識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError answers a missing or foreign run the same way, so callers cannot probe which identifiers exist.
func (backtestRunController *BacktestRunController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrBacktestRunNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrBacktestRunAlreadyEnded) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

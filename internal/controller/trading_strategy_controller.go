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

// TradingStrategyController exposes the trading strategy use cases over HTTP.
type TradingStrategyController struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyController(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyController {
	return &TradingStrategyController{tradingStrategyApplication: tradingStrategyApplication}
}

// CreateTradingStrategy handles POST /trading-strategies.
func (tradingStrategyController *TradingStrategyController) CreateTradingStrategy(
	ginContext *gin.Context,
) {
	var tradingStrategyRequest models.TradingStrategyRequest

	if bindError := ginContext.ShouldBindJSON(&tradingStrategyRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	tradingStrategyDto, err := tradingStrategyController.tradingStrategyApplication.CreateTradingStrategy(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyRequest.ToWriteDto(0))
	if err != nil {
		tradingStrategyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, tradingStrategyDto)
}

// ListTradingStrategies handles GET /trading-strategies.
func (tradingStrategyController *TradingStrategyController) ListTradingStrategies(
	ginContext *gin.Context,
) {
	tradingStrategyDtos, err := tradingStrategyController.tradingStrategyApplication.ListTradingStrategies(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		tradingStrategyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, tradingStrategyDtos)
}

// GetTradingStrategy handles GET /trading-strategies/:id, and serves owners only.
func (tradingStrategyController *TradingStrategyController) GetTradingStrategy(
	ginContext *gin.Context,
) {
	id, idIsReadable := tradingStrategyController.readID(ginContext)
	if !idIsReadable {
		return
	}

	tradingStrategyDto, err := tradingStrategyController.tradingStrategyApplication.GetTradingStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		tradingStrategyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, tradingStrategyDto)
}

// UpdateTradingStrategy handles PUT /trading-strategies/:id.
func (tradingStrategyController *TradingStrategyController) UpdateTradingStrategy(
	ginContext *gin.Context,
) {
	id, idIsReadable := tradingStrategyController.readID(ginContext)
	if !idIsReadable {
		return
	}

	var tradingStrategyRequest models.TradingStrategyRequest

	if bindError := ginContext.ShouldBindJSON(&tradingStrategyRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	tradingStrategyDto, err := tradingStrategyController.tradingStrategyApplication.UpdateTradingStrategy(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		tradingStrategyRequest.ToWriteDto(id))
	if err != nil {
		tradingStrategyController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, tradingStrategyDto)
}

// DeleteTradingStrategy handles DELETE /trading-strategies/:id.
func (tradingStrategyController *TradingStrategyController) DeleteTradingStrategy(
	ginContext *gin.Context,
) {
	id, idIsReadable := tradingStrategyController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := tradingStrategyController.tradingStrategyApplication.DeleteTradingStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		tradingStrategyController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readID reads which trading strategy the path names.
func (tradingStrategyController *TradingStrategyController) readID(
	ginContext *gin.Context,
) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "交易策略識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
//
// A strategy script that cannot be seen comes back as this feature's own not-found,
// not as the strategy script one. A caller naming a script they may not use is told
// the same thing whichever way they reached it, and nothing about the answer says
// whether that script exists.
//
// A rewrite blocked by a running bot and a delete blocked by any bot are both
// conflicts rather than refusals of the request: nothing about what was sent is
// wrong, and the same request succeeds once those bots are dealt with.
func (tradingStrategyController *TradingStrategyController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrTradingStrategyValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrTradingStrategyNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrTradingStrategyNameConflict) ||
		errors.Is(err, domains.ErrTradingStrategyBotRunning) ||
		errors.Is(err, domains.ErrTradingStrategyInUse) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

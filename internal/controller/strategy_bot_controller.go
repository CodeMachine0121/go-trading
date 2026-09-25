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

type StrategyBotController struct {
	strategyBotApplication *application.StrategyBotApplication
	// Used only by the run-now route, which deliberately shares the scheduled scan's path so a manual round behaves exactly like an automatic one.
	strategyBotRunApplication *application.StrategyBotRunApplication
}

func NewStrategyBotController(
	strategyBotApplication *application.StrategyBotApplication,
	strategyBotRunApplication *application.StrategyBotRunApplication,
) *StrategyBotController {
	return &StrategyBotController{
		strategyBotApplication:    strategyBotApplication,
		strategyBotRunApplication: strategyBotRunApplication,
	}
}

// CreateStrategyBot handles POST /strategy-bots.
func (strategyBotController *StrategyBotController) CreateStrategyBot(ginContext *gin.Context) {
	var strategyBotRequest models.StrategyBotRequest

	if bindError := ginContext.ShouldBindJSON(&strategyBotRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotApplication.CreateStrategyBot(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		strategyBotRequest.ToWriteDto(0))
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, strategyBotDto)
}

// ListStrategyBots handles GET /strategy-bots, optionally filtered by the marketDataKind query.
func (strategyBotController *StrategyBotController) ListStrategyBots(ginContext *gin.Context) {
	strategyBotDtos, err := strategyBotController.strategyBotApplication.ListStrategyBots(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext),
		ginContext.Query("marketDataKind"))
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDtos)
}

// GetStrategyBot handles GET /strategy-bots/:id, and serves owners only.
func (strategyBotController *StrategyBotController) GetStrategyBot(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotApplication.GetStrategyBot(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDto)
}

// UpdateStrategyBot handles PUT /strategy-bots/:id.
func (strategyBotController *StrategyBotController) UpdateStrategyBot(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	var strategyBotRequest models.StrategyBotRequest

	if bindError := ginContext.ShouldBindJSON(&strategyBotRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotApplication.UpdateStrategyBot(
		ginContext.Request.Context(),
		middlewares.CurrentUserID(ginContext),
		strategyBotRequest.ToWriteDto(id))
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDto)
}

// DeleteStrategyBot handles DELETE /strategy-bots/:id, running or not.
func (strategyBotController *StrategyBotController) DeleteStrategyBot(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyBotController.strategyBotApplication.DeleteStrategyBot(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// StartStrategyBot handles POST /strategy-bots/:id/run.
// Start and stop create and delete one subresource, so repeating either is harmless.
func (strategyBotController *StrategyBotController) StartStrategyBot(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotApplication.StartStrategyBot(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDto)
}

// StopStrategyBot handles DELETE /strategy-bots/:id/run.
func (strategyBotController *StrategyBotController) StopStrategyBot(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotApplication.StopStrategyBot(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDto)
}

// readID answers a bad request itself for zero, unreadable or wider-than-column IDs (a wider read would wrap onto another bot); false means the response was already sent.
func (strategyBotController *StrategyBotController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "機器人識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
// An invisible strategy script answers as this feature's own not-found, so the response never reveals whether the script exists.
func (strategyBotController *StrategyBotController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrStrategyBotValidation) ||
		errors.Is(err, domains.ErrStrategyBotDeliveryNotConfigured) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyBotNotFound) ||
		errors.Is(err, domains.ErrTradingStrategyNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyBotNameConflict) ||
		errors.Is(err, domains.ErrStrategyBotRunning) ||
		errors.Is(err, domains.ErrStrategyBotAlreadyRunningARound) ||
		errors.Is(err, domains.ErrStrategyBotRunningLimitReached) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

// RunRoundNow handles POST /strategy-bots/:id/runs: run one round right now.
// Answers with the updated bot rather than the round, which is visible in the run history.
func (strategyBotController *StrategyBotController) RunRoundNow(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	strategyBotDto, err := strategyBotController.strategyBotRunApplication.RunRoundNow(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyBotDto)
}

// ListRunRecords handles GET /strategy-bots/:id/runs: what this bot has been doing.
func (strategyBotController *StrategyBotController) ListRunRecords(ginContext *gin.Context) {
	id, idIsReadable := strategyBotController.readID(ginContext)
	if !idIsReadable {
		return
	}

	runRecordDtos, err := strategyBotController.strategyBotApplication.ListRunRecords(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyBotController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, runRecordDtos)
}

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

// StrategyBotController exposes the strategy bot use cases over HTTP.
type StrategyBotController struct {
	strategyBotApplication *application.StrategyBotApplication
}

func NewStrategyBotController(
	strategyBotApplication *application.StrategyBotApplication,
) *StrategyBotController {
	return &StrategyBotController{strategyBotApplication: strategyBotApplication}
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

// ListStrategyBots handles GET /strategy-bots.
func (strategyBotController *StrategyBotController) ListStrategyBots(ginContext *gin.Context) {
	strategyBotDtos, err := strategyBotController.strategyBotApplication.ListStrategyBots(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
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
//
// Starting and stopping are one subresource's existence rather than two verbs,
// which is what makes pressing either twice harmless without anything having to
// remember that it should be.
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

// readID reads the bot identifier out of the path, answering the caller with a bad
// request when it is not one. The second return value says whether the handler may
// carry on — a handler that gets false has already had its answer sent.
//
// Zero is refused along with anything unreadable, and so is anything wider than the
// column holds: reading it wider would wrap an oversized number into a small one and
// answer for whichever bot that landed on.
func (strategyBotController *StrategyBotController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "機器人識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
//
// A strategy that cannot be seen comes back as this feature's own not-found, not as
// the strategy one. A caller naming a strategy they may not use is told the same
// thing whichever way they reached it, and nothing about the answer says whether
// that strategy exists.
func (strategyBotController *StrategyBotController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrStrategyBotValidation) ||
		errors.Is(err, domains.ErrStrategyBotDeliveryNotConfigured) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyBotNotFound) || errors.Is(err, domains.ErrStrategyNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyBotNameConflict) ||
		errors.Is(err, domains.ErrStrategyBotRunning) ||
		errors.Is(err, domains.ErrStrategyBotRunningLimitReached) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

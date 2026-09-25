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

type StrategyScriptController struct {
	strategyScriptApplication *application.StrategyScriptApplication
}

func NewStrategyScriptController(strategyScriptApplication *application.StrategyScriptApplication) *StrategyScriptController {
	return &StrategyScriptController{strategyScriptApplication: strategyScriptApplication}
}

// CreateStrategyScript handles POST /strategy-scripts.
func (strategyScriptController *StrategyScriptController) CreateStrategyScript(ginContext *gin.Context) {
	var strategyScriptRequest models.StrategyScriptRequest

	if bindError := ginContext.ShouldBindJSON(&strategyScriptRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyScriptDto, err := strategyScriptController.strategyScriptApplication.CreateStrategyScript(ginContext.Request.Context(),
		strategyScriptRequest.ToWriteDto(0, middlewares.CurrentUserID(ginContext)))
	if err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, strategyScriptDto)
}

// ListAvailableStrategyScripts handles GET /strategy-scripts, returning owned and adopted scripts as two lists since adopted ones carry no script.
func (strategyScriptController *StrategyScriptController) ListAvailableStrategyScripts(ginContext *gin.Context) {
	availableStrategyScriptsDto, err := strategyScriptController.strategyScriptApplication.ListAvailableStrategyScripts(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, availableStrategyScriptsDto)
}

// GetStrategyScript handles GET /strategy-scripts/:id for owners only; others read published scripts via the marketplace, without the script.
func (strategyScriptController *StrategyScriptController) GetStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptController.readID(ginContext)
	if !idIsReadable {
		return
	}

	strategyScriptDto, err := strategyScriptController.strategyScriptApplication.GetStrategyScript(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyScriptDto)
}

// UpdateStrategyScript handles PUT /strategy-scripts/:id.
func (strategyScriptController *StrategyScriptController) UpdateStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptController.readID(ginContext)
	if !idIsReadable {
		return
	}

	var strategyScriptRequest models.StrategyScriptRequest

	if bindError := ginContext.ShouldBindJSON(&strategyScriptRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyScriptDto, err := strategyScriptController.strategyScriptApplication.UpdateStrategyScript(ginContext.Request.Context(),
		strategyScriptRequest.ToWriteDto(id, middlewares.CurrentUserID(ginContext)))
	if err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, strategyScriptDto)
}

// DeleteStrategyScript handles DELETE /strategy-scripts/:id.
func (strategyScriptController *StrategyScriptController) DeleteStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyScriptController.strategyScriptApplication.DeleteStrategyScript(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readID answers a bad request itself for zero or IDs wider than the column (a wider read would wrap onto another script, an unchecked one would surface as a storage failure); false means the response was already sent.
func (strategyScriptController *StrategyScriptController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "策略腳本識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps only the strategy script's own errors.
func (strategyScriptController *StrategyScriptController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrStrategyScriptValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyScriptNameConflict) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

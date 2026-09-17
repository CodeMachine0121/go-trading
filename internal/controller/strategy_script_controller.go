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

// StrategyScriptController exposes the saved strategy script use cases over HTTP.
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

// ListAvailableStrategyScripts handles GET /strategy-scripts: what this caller picks from,
// which is their own strategy scripts plus the ones they took off the marketplace.
//
// The two come back as two lists rather than one, because the adopted ones carry no
// script and there is no single shape that could hold both.
func (strategyScriptController *StrategyScriptController) ListAvailableStrategyScripts(ginContext *gin.Context) {
	availableStrategyScriptsDto, err := strategyScriptController.strategyScriptApplication.ListAvailableStrategyScripts(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		strategyScriptController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, availableStrategyScriptsDto)
}

// GetStrategyScript handles GET /strategy-scripts/:id, and serves owners only. Somebody else's
// published strategy script is read from the marketplace, which hands back a different
// shape — one without a script.
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

// readID reads the strategy script identifier out of the path, answering the caller with a
// bad request when it is not one. The second return value says whether the handler
// may carry on — a handler that gets false has already had its answer sent.
//
// Zero is refused along with anything unreadable: no strategy script carries it, and it is
// the very value that means "a strategy script that does not exist yet" further in, so
// letting it through would ask the storage layer to rewrite nothing in particular.
//
// It is read at the width an identifier is actually held in, and refused above what
// the column can hold. Reading it wider would wrap a number too large to hold into a
// small one and answer for whichever strategy script that landed on; letting an oversized
// one through instead reaches the database and comes back as a storage failure,
// which reads as "something broke" when the truth is that no strategy script has that
// identifier.
func (strategyScriptController *StrategyScriptController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "策略腳本識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it. It
// knows only the strategy script's own errors: a caller must not have to recognise a K
// candle's failure to find out its strategy script was rejected.
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

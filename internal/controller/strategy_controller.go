package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// StrategyController exposes the saved strategy use cases over HTTP.
type StrategyController struct {
	strategyApplication *application.StrategyApplication
}

func NewStrategyController(strategyApplication *application.StrategyApplication) *StrategyController {
	return &StrategyController{strategyApplication: strategyApplication}
}

// CreateStrategy handles POST /strategies.
func (strategyController *StrategyController) CreateStrategy(context *gin.Context) {
	var strategyRequest models.StrategyRequest

	if bindError := context.ShouldBindJSON(&strategyRequest); bindError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyDto, err := strategyController.strategyApplication.CreateStrategy(
		strategyRequest.ToWriteDto(0))
	if err != nil {
		strategyController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusCreated, strategyDto)
}

// ListStrategies handles GET /strategies.
func (strategyController *StrategyController) ListStrategies(context *gin.Context) {
	strategyDtos, err := strategyController.strategyApplication.ListStrategies()
	if err != nil {
		strategyController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, strategyDtos)
}

// GetStrategy handles GET /strategies/:id.
func (strategyController *StrategyController) GetStrategy(context *gin.Context) {
	id, idIsReadable := strategyController.readID(context)
	if !idIsReadable {
		return
	}

	strategyDto, err := strategyController.strategyApplication.GetStrategy(id)
	if err != nil {
		strategyController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, strategyDto)
}

// UpdateStrategy handles PUT /strategies/:id.
func (strategyController *StrategyController) UpdateStrategy(context *gin.Context) {
	id, idIsReadable := strategyController.readID(context)
	if !idIsReadable {
		return
	}

	var strategyRequest models.StrategyRequest

	if bindError := context.ShouldBindJSON(&strategyRequest); bindError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	strategyDto, err := strategyController.strategyApplication.UpdateStrategy(
		strategyRequest.ToWriteDto(id))
	if err != nil {
		strategyController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, strategyDto)
}

// DeleteStrategy handles DELETE /strategies/:id.
func (strategyController *StrategyController) DeleteStrategy(context *gin.Context) {
	id, idIsReadable := strategyController.readID(context)
	if !idIsReadable {
		return
	}

	if err := strategyController.strategyApplication.DeleteStrategy(id); err != nil {
		strategyController.respondWithError(context, err)
		return
	}

	context.Status(http.StatusNoContent)
}

// readID reads the strategy identifier out of the path, answering the caller with a
// bad request when it is not one. The second return value says whether the handler
// may carry on — a handler that gets false has already had its answer sent.
//
// Zero is refused along with anything unreadable: no strategy carries it, and it is
// the very value that means "a strategy that does not exist yet" further in, so
// letting it through would ask the storage layer to rewrite nothing in particular.
//
// It is read at the width an identifier is actually held in. Reading it wider and
// narrowing afterwards would wrap a number too large to hold into a small one, and
// answer for whichever strategy that landed on.
func (strategyController *StrategyController) readID(context *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(context.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 {
		context.JSON(http.StatusBadRequest, gin.H{"message": "策略識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it. It
// knows only the strategy's own errors: a caller must not have to recognise a K
// candle's failure to find out its strategy was rejected.
func (strategyController *StrategyController) respondWithError(context *gin.Context, err error) {
	if errors.Is(err, domains.ErrStrategyValidation) {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyNameConflict) {
		context.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	context.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

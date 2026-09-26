package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

type StrategyScriptMarketplaceController struct {
	strategyScriptMarketplaceApplication *application.StrategyScriptMarketplaceApplication
}

func NewStrategyScriptMarketplaceController(
	strategyScriptMarketplaceApplication *application.StrategyScriptMarketplaceApplication,
) *StrategyScriptMarketplaceController {
	return &StrategyScriptMarketplaceController{strategyScriptMarketplaceApplication: strategyScriptMarketplaceApplication}
}

// BrowseMarketplace handles GET /marketplace/strategy-scripts.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) BrowseMarketplace(ginContext *gin.Context) {
	publishedStrategyScriptDtos, err := strategyScriptMarketplaceController.strategyScriptMarketplaceApplication.BrowseMarketplace(
		ginContext.Request.Context())
	if err != nil {
		strategyScriptMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, publishedStrategyScriptDtos)
}

// PublishStrategyScript handles POST /strategy-scripts/:id/publication.
// Publishing sets a state, so repeating it is idempotent.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) PublishStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyScriptMarketplaceController.strategyScriptMarketplaceApplication.PublishStrategyScript(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyScriptMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// WithdrawStrategyScript handles DELETE /strategy-scripts/:id/publication.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) WithdrawStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyScriptMarketplaceController.strategyScriptMarketplaceApplication.WithdrawStrategyScript(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyScriptMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// AdoptStrategyScript handles POST /marketplace/strategy-scripts/:id/adoption.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) AdoptStrategyScript(ginContext *gin.Context) {
	id, idIsReadable := strategyScriptMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyScriptMarketplaceController.strategyScriptMarketplaceApplication.AdoptStrategyScript(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyScriptMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readID answers a bad request itself when the path ID is invalid; false means the response was already sent.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "策略腳本識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
// Missing, foreign and unpublished scripts all arrive as one error and answer 404, so strangers cannot probe which identifiers exist.
func (strategyScriptMarketplaceController *StrategyScriptMarketplaceController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrStrategyScriptNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	// The copy's name is already held, or a copy is being published again.
	if errors.Is(err, domains.ErrStrategyScriptNameConflict) ||
		errors.Is(err, domains.ErrStrategyScriptFromMarketplace) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
}

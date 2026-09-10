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

// StrategyMarketplaceController exposes the shared shelf over HTTP: what is on it,
// putting one's own strategy there or taking it back, and keeping a selection.
type StrategyMarketplaceController struct {
	strategyMarketplaceApplication *application.StrategyMarketplaceApplication
}

func NewStrategyMarketplaceController(
	strategyMarketplaceApplication *application.StrategyMarketplaceApplication,
) *StrategyMarketplaceController {
	return &StrategyMarketplaceController{strategyMarketplaceApplication: strategyMarketplaceApplication}
}

// BrowseMarketplace handles GET /marketplace/strategies.
func (strategyMarketplaceController *StrategyMarketplaceController) BrowseMarketplace(ginContext *gin.Context) {
	publishedStrategyDtos, err := strategyMarketplaceController.strategyMarketplaceApplication.BrowseMarketplace(
		ginContext.Request.Context())
	if err != nil {
		strategyMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, publishedStrategyDtos)
}

// PublishStrategy handles POST /strategies/:id/publication.
//
// Publishing is stated as the state to end in rather than as an event, so pressing
// it twice says the same thing as pressing it once and answers the same way.
func (strategyMarketplaceController *StrategyMarketplaceController) PublishStrategy(ginContext *gin.Context) {
	id, idIsReadable := strategyMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyMarketplaceController.strategyMarketplaceApplication.PublishStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// WithdrawStrategy handles DELETE /strategies/:id/publication.
func (strategyMarketplaceController *StrategyMarketplaceController) WithdrawStrategy(ginContext *gin.Context) {
	id, idIsReadable := strategyMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyMarketplaceController.strategyMarketplaceApplication.WithdrawStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// AdoptStrategy handles POST /marketplace/strategies/:id/adoption.
func (strategyMarketplaceController *StrategyMarketplaceController) AdoptStrategy(ginContext *gin.Context) {
	id, idIsReadable := strategyMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyMarketplaceController.strategyMarketplaceApplication.AdoptStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// AbandonStrategy handles DELETE /marketplace/strategies/:id/adoption.
func (strategyMarketplaceController *StrategyMarketplaceController) AbandonStrategy(ginContext *gin.Context) {
	id, idIsReadable := strategyMarketplaceController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := strategyMarketplaceController.strategyMarketplaceApplication.AbandonStrategy(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		strategyMarketplaceController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readID reads the strategy identifier out of the path, answering the caller with a
// bad request when it is not one. The second return value says whether the handler
// may carry on.
func (strategyMarketplaceController *StrategyMarketplaceController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "策略識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
//
// There is one refusal to recognise, and that is the whole design: a strategy that
// is not there, one that belongs to somebody else, and one that is not on the
// marketplace all arrive here as the same error and leave as the same 404. Adding a
// 403 for "yours it is not" would hand a stranger a way to ask which identifiers
// exist.
func (strategyMarketplaceController *StrategyMarketplaceController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrStrategyNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
}

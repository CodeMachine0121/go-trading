package controller

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type AssistantPendingRevisionController struct {
	assistantRevisionApplication *application.AssistantRevisionApplication
}

func NewAssistantPendingRevisionController(
	assistantRevisionApplication *application.AssistantRevisionApplication,
) *AssistantPendingRevisionController {
	return &AssistantPendingRevisionController{assistantRevisionApplication: assistantRevisionApplication}
}

// ConfirmPendingRevision handles POST /chat/pending-revisions/:id/confirm.
func (assistantPendingRevisionController *AssistantPendingRevisionController) ConfirmPendingRevision(
	ginContext *gin.Context,
) {
	assistantPendingRevisionController.resolve(ginContext,
		assistantPendingRevisionController.assistantRevisionApplication.ConfirmPendingRevision)
}

// RejectPendingRevision handles POST /chat/pending-revisions/:id/reject.
func (assistantPendingRevisionController *AssistantPendingRevisionController) RejectPendingRevision(
	ginContext *gin.Context,
) {
	assistantPendingRevisionController.resolve(ginContext,
		assistantPendingRevisionController.assistantRevisionApplication.RejectPendingRevision)
}

// resolve is shared by confirming and rejecting, which differ only in the use case they call.
func (assistantPendingRevisionController *AssistantPendingRevisionController) resolve(
	ginContext *gin.Context,
	resolution func(executionContext context.Context, viewerID uint, id uint) (dto.AssistantPendingRevisionDto, error),
) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "待確認修改識別碼必須是正整數"})
		return
	}

	pendingRevisionDto, err := resolution(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), uint(id))
	if err != nil {
		assistantPendingRevisionController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, pendingRevisionDto)
}

// respondWithError also maps the rewrite's own refusals, since confirming carries the rewrite out.
func (assistantPendingRevisionController *AssistantPendingRevisionController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrAssistantPendingRevisionNotFound) ||
		errors.Is(err, domains.ErrStrategyScriptNotFound) ||
		errors.Is(err, domains.ErrTradingStrategyNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrAssistantPendingRevisionResolved) ||
		errors.Is(err, domains.ErrAssistantPendingRevisionStale) ||
		errors.Is(err, domains.ErrStrategyScriptNameConflict) ||
		errors.Is(err, domains.ErrStrategyScriptBotRunning) ||
		errors.Is(err, domains.ErrStrategyScriptFromMarketplace) ||
		errors.Is(err, domains.ErrTradingStrategyNameConflict) ||
		errors.Is(err, domains.ErrTradingStrategyBotRunning) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrStrategyScriptValidation) ||
		errors.Is(err, domains.ErrTradingStrategyValidation) ||
		errors.Is(err, domains.ErrAssistantQueryArgument) ||
		errors.Is(err, domains.ErrStrategyScriptNotYours) ||
		errors.Is(err, domains.ErrStrategyScriptMarketDataKindMismatch) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

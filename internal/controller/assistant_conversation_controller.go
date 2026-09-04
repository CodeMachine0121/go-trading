package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

// AssistantConversationController exposes the conversation use cases over HTTP.
type AssistantConversationController struct {
	assistantConversationApplication *application.AssistantConversationApplication
}

func NewAssistantConversationController(
	assistantConversationApplication *application.AssistantConversationApplication,
) *AssistantConversationController {
	return &AssistantConversationController{
		assistantConversationApplication: assistantConversationApplication,
	}
}

// Ask handles POST /chat.
func (assistantConversationController *AssistantConversationController) Ask(ginContext *gin.Context) {
	var assistantAskRequest models.AssistantAskRequest

	if bindError := ginContext.ShouldBindJSON(&assistantAskRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	answerDto, err := assistantConversationController.assistantConversationApplication.Ask(
		ginContext.Request.Context(), assistantAskRequest.ToAskDto())
	if err != nil {
		assistantConversationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, answerDto)
}

// ListConversations handles GET /chat/conversations.
func (assistantConversationController *AssistantConversationController) ListConversations(ginContext *gin.Context) {
	summaryDtos, err := assistantConversationController.assistantConversationApplication.ListConversations(
		ginContext.Request.Context())
	if err != nil {
		assistantConversationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, summaryDtos)
}

// GetConversation handles GET /chat/conversations/:id.
func (assistantConversationController *AssistantConversationController) GetConversation(ginContext *gin.Context) {
	id, idIsReadable := assistantConversationController.readID(ginContext)
	if !idIsReadable {
		return
	}

	conversationDto, err := assistantConversationController.assistantConversationApplication.GetConversation(
		ginContext.Request.Context(), id)
	if err != nil {
		assistantConversationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, conversationDto)
}

// readID reads the conversation identifier out of the path, answering the caller with
// a bad request when it is not one. The second return value says whether the handler
// may carry on — a handler that gets false has already had its answer sent.
//
// Zero is refused along with anything unreadable: no conversation carries it, and it
// is the very value that means "no conversation yet" further in.
func (assistantConversationController *AssistantConversationController) readID(
	ginContext *gin.Context,
) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "對話識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError maps a domain error onto the status code that reports it.
//
// The four are deliberately four different codes, because what the reader has to do
// about them differs: fix the question, name a conversation that exists, wait for
// tomorrow, or try again shortly. Collapsing the last two into one would leave
// somebody retrying a refusal that will still be there in an hour.
func (assistantConversationController *AssistantConversationController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrAssistantAskEmpty) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrConversationNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrDailyUsageAllowanceExhausted) {
		ginContext.JSON(http.StatusTooManyRequests, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrAssistantUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

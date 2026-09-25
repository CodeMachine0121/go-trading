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

// Ask handles POST /chat, answering 202 because the answer is only reserved, not yet written.
func (assistantConversationController *AssistantConversationController) Ask(ginContext *gin.Context) {
	var assistantAskRequest models.AssistantAskRequest

	if bindError := ginContext.ShouldBindJSON(&assistantAskRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	answerStartedDto, err := assistantConversationController.assistantConversationApplication.Ask(
		ginContext.Request.Context(), assistantAskRequest.ToAskDto(middlewares.CurrentUserID(ginContext)))
	if err != nil {
		assistantConversationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, answerStartedDto)
}

// ListConversations handles GET /chat/conversations.
func (assistantConversationController *AssistantConversationController) ListConversations(ginContext *gin.Context) {
	summaryDtos, err := assistantConversationController.assistantConversationApplication.ListConversations(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
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
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		assistantConversationController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, conversationDto)
}

// readID answers a bad request itself when the path ID is unreadable or zero (zero means "no conversation yet"); false means the response was already sent.
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

// respondWithError maps each domain error to a distinct status because the caller's remedy differs; assistant failures surface on the exchange itself, not here.
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
	if errors.Is(err, domains.ErrAssistantAnswerInProgress) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

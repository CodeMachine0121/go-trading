package controller

import (
	"errors"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/gin-gonic/gin"
)

type TelegramDeliveryController struct {
	telegramDeliveryApplication *application.TelegramDeliveryApplication
}

func NewTelegramDeliveryController(
	telegramDeliveryApplication *application.TelegramDeliveryApplication,
) *TelegramDeliveryController {
	return &TelegramDeliveryController{telegramDeliveryApplication: telegramDeliveryApplication}
}

// GetDeliverySetting handles GET /users/me/telegram-delivery.
// No setting answers 200 with a setting saying so, since that is the resource's ordinary state rather than a 404.
func (telegramDeliveryController *TelegramDeliveryController) GetDeliverySetting(
	ginContext *gin.Context,
) {
	telegramDeliveryDto, err := telegramDeliveryController.telegramDeliveryApplication.
		GetDeliverySetting(ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		telegramDeliveryController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, telegramDeliveryDto)
}

// SaveDeliverySetting handles PUT /users/me/telegram-delivery.
// PUT because each person has at most one setting and resending leaves the same one.
func (telegramDeliveryController *TelegramDeliveryController) SaveDeliverySetting(
	ginContext *gin.Context,
) {
	var telegramDeliveryRequest models.TelegramDeliveryRequest

	if bindError := ginContext.ShouldBindJSON(&telegramDeliveryRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	telegramDeliveryDto, err := telegramDeliveryController.telegramDeliveryApplication.
		SaveDeliverySetting(
			ginContext.Request.Context(),
			middlewares.CurrentUserID(ginContext),
			telegramDeliveryRequest.ToTelegramDeliveryWriteDto(),
		)
	if err != nil {
		telegramDeliveryController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, telegramDeliveryDto)
}

// RemoveDeliverySetting handles DELETE /users/me/telegram-delivery.
// Answers 204 whether or not anything was removed, since the requested state holds either way.
func (telegramDeliveryController *TelegramDeliveryController) RemoveDeliverySetting(
	ginContext *gin.Context,
) {
	if err := telegramDeliveryController.telegramDeliveryApplication.RemoveDeliverySetting(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext)); err != nil {
		telegramDeliveryController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// SendTestMessage handles POST /users/me/telegram-delivery/test-message.
// A message Telegram rejects still answers 200 with the named reason in the body, since the send was attempted; only a send that never happened fails.
func (telegramDeliveryController *TelegramDeliveryController) SendTestMessage(
	ginContext *gin.Context,
) {
	var testMessageRequest models.TestMessageRequest

	if bindError := ginContext.ShouldBindJSON(&testMessageRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	testMessageResultDto, err := telegramDeliveryController.telegramDeliveryApplication.
		SendTestMessage(
			ginContext.Request.Context(),
			middlewares.CurrentUserID(ginContext),
			testMessageRequest.ToTestMessageDto(),
		)
	if err != nil {
		telegramDeliveryController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, testMessageResultDto)
}

// respondWithError maps only this feature's own errors.
func (telegramDeliveryController *TelegramDeliveryController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrTelegramDeliveryValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// 409 rather than 400: a setup step is missing, nothing the caller sent is wrong.
	if errors.Is(err, domains.ErrTelegramDeliveryNotConfigured) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	// A missing sealing key is a system failure; the caller's token was fine.
	if errors.Is(err, domains.ErrSecretSealUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

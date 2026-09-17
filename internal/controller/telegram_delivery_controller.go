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

// TelegramDeliveryController exposes over HTTP the use cases about where this system
// speaks to somebody.
type TelegramDeliveryController struct {
	telegramDeliveryApplication *application.TelegramDeliveryApplication
}

func NewTelegramDeliveryController(
	telegramDeliveryApplication *application.TelegramDeliveryApplication,
) *TelegramDeliveryController {
	return &TelegramDeliveryController{telegramDeliveryApplication: telegramDeliveryApplication}
}

// GetDeliverySetting handles GET /users/me/telegram-delivery.
//
// Never having set one up answers 200 with a setting that says so, rather than 404.
// It is not a missing resource — it is the ordinary state of the resource, and a
// caller handed 404 would have to decide which 404s mean nothing is wrong.
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
//
// PUT rather than POST because there is at most one of these per person and sending
// it twice leaves the same single setting behind — which is what PUT promises and
// POST does not.
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
//
// It answers 204 whether or not there was anything to remove, because "this system
// can no longer reach you" is true either way — and a caller told otherwise would
// retry to reach a state it is already in.
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
//
// A message Telegram would not take still answers 200. That looks wrong for about a
// second and then stops: the request asked this system to try sending and report
// back, and it did exactly that. The reason travels in the body as a named value
// rather than as a status code for two reasons — there are four of them and no four
// status codes mean these four things, and a caller has to tell them apart to say
// which box the person should go and fix.
//
// What does fail is a send that never happened: a message breaking a rule, no
// setting to send with, or no key to open the token.
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

// respondWithError maps this feature's own errors onto the status codes that report
// them. It knows only this feature's: a caller must not have to recognise a
// strategy script's failure to find out their message was blank.
func (telegramDeliveryController *TelegramDeliveryController) respondWithError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrTelegramDeliveryValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	// Nothing the caller sent is wrong; what is missing is a step they have not
	// taken yet. 409 rather than 400 so that "fix this box" and "go and do that
	// first" are told apart without reading the sentence.
	if errors.Is(err, domains.ErrTelegramDeliveryNotConfigured) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	// Having nothing to lock a secret with is the system being unable to do its
	// job, not the caller having asked wrongly — their token was fine and there is
	// nothing they can change. Saying so is what stops somebody generating four
	// bot tokens before giving up.
	if errors.Is(err, domains.ErrSecretSealUnavailable) {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

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

type TradeJournalSettingController struct {
	tradeJournalSettingApplication *application.TradeJournalSettingApplication
}

func NewTradeJournalSettingController(
	tradeJournalSettingApplication *application.TradeJournalSettingApplication,
) *TradeJournalSettingController {
	return &TradeJournalSettingController{tradeJournalSettingApplication: tradeJournalSettingApplication}
}

func (settingController *TradeJournalSettingController) GetSetting(ginContext *gin.Context) {
	settingDto, err := settingController.tradeJournalSettingApplication.GetSetting(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, settingDto)
}

func (settingController *TradeJournalSettingController) SaveFeeRates(ginContext *gin.Context) {
	var settingRequest models.TradeJournalSettingRequest
	if bindError := ginContext.ShouldBindJSON(&settingRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	settingDto, err := settingController.tradeJournalSettingApplication.SaveFeeRates(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), settingRequest.ToWriteDto())
	if err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, settingDto)
}

func (settingController *TradeJournalSettingController) ListTags(ginContext *gin.Context) {
	tagDtos, err := settingController.tradeJournalSettingApplication.ListTags(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, tagDtos)
}

func (settingController *TradeJournalSettingController) CreateTag(ginContext *gin.Context) {
	var tagRequest models.TradeTagRequest
	if bindError := ginContext.ShouldBindJSON(&tagRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	tagDto, err := settingController.tradeJournalSettingApplication.CreateTag(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), tagRequest.ToWriteDto())
	if err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, tagDto)
}

func (settingController *TradeJournalSettingController) RenameTag(ginContext *gin.Context) {
	id, idIsReadable := settingController.readID(ginContext)
	if !idIsReadable {
		return
	}

	var renameRequest models.TradeTagRenameRequest
	if bindError := ginContext.ShouldBindJSON(&renameRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	tagDto, err := settingController.tradeJournalSettingApplication.RenameTag(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, renameRequest.Name)
	if err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, tagDto)
}

func (settingController *TradeJournalSettingController) DeleteTag(ginContext *gin.Context) {
	id, idIsReadable := settingController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := settingController.tradeJournalSettingApplication.DeleteTag(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		settingController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

func (settingController *TradeJournalSettingController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "標籤識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

func (settingController *TradeJournalSettingController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrTradeJournalSettingValidation) || errors.Is(err, domains.ErrTradeTagValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrTradeTagNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrTradeTagNameConflict) || errors.Is(err, domains.ErrTradeTagInUse) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

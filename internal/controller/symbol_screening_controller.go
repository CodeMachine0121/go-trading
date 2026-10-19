package controller

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// SymbolScreeningController serves the administrator-only screening area: profiles and runs.
type SymbolScreeningController struct {
	screeningApplication *application.SymbolScreeningApplication
}

func NewSymbolScreeningController(screeningApplication *application.SymbolScreeningApplication) *SymbolScreeningController {
	return &SymbolScreeningController{screeningApplication: screeningApplication}
}

func (screeningController *SymbolScreeningController) ListProfiles(ginContext *gin.Context) {
	profileDtos, err := screeningController.screeningApplication.ListProfiles(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext))
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, profileDtos)
}

func (screeningController *SymbolScreeningController) GetProfile(ginContext *gin.Context) {
	id, idIsReadable := screeningController.readID(ginContext)
	if !idIsReadable {
		return
	}

	profileDto, err := screeningController.screeningApplication.GetProfile(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, profileDto)
}

func (screeningController *SymbolScreeningController) CreateProfile(ginContext *gin.Context) {
	var profileRequest models.SymbolScreeningProfileRequest
	if bindError := ginContext.ShouldBindJSON(&profileRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "篩選設定的格式不正確"})
		return
	}

	profileDto, err := screeningController.screeningApplication.CreateProfile(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), profileRequest.ToWriteDto())
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, profileDto)
}

func (screeningController *SymbolScreeningController) UpdateProfile(ginContext *gin.Context) {
	id, idIsReadable := screeningController.readID(ginContext)
	if !idIsReadable {
		return
	}
	var profileRequest models.SymbolScreeningProfileRequest
	if bindError := ginContext.ShouldBindJSON(&profileRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "篩選設定的格式不正確"})
		return
	}

	profileDto, err := screeningController.screeningApplication.UpdateProfile(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, profileRequest.ToWriteDto())
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, profileDto)
}

func (screeningController *SymbolScreeningController) DeleteProfile(ginContext *gin.Context) {
	id, idIsReadable := screeningController.readID(ginContext)
	if !idIsReadable {
		return
	}

	if err := screeningController.screeningApplication.DeleteProfile(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// SubmitRun answers 202 with the queued run; screening happens in the background.
func (screeningController *SymbolScreeningController) SubmitRun(ginContext *gin.Context) {
	var runRequest models.SymbolScreeningRunRequest
	if bindError := ginContext.ShouldBindJSON(&runRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "篩選要求的格式不正確"})
		return
	}

	runDto, err := screeningController.screeningApplication.SubmitRun(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), runRequest.ToSubmissionDto())
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusAccepted, runDto)
}

func (screeningController *SymbolScreeningController) ListRuns(ginContext *gin.Context) {
	limit, _ := strconv.Atoi(ginContext.Query("limit"))

	pageDto, err := screeningController.screeningApplication.ListRuns(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext),
		dto.SymbolScreeningRunListQueryDto{Status: ginContext.Query("status"), Limit: limit})
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, pageDto)
}

func (screeningController *SymbolScreeningController) GetRun(ginContext *gin.Context) {
	id, idIsReadable := screeningController.readID(ginContext)
	if !idIsReadable {
		return
	}
	waitSeconds, _ := strconv.Atoi(ginContext.Query("waitSeconds"))

	detailDto, err := screeningController.screeningApplication.GetRun(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, time.Duration(waitSeconds)*time.Second)
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, detailDto)
}

func (screeningController *SymbolScreeningController) CancelRun(ginContext *gin.Context) {
	id, idIsReadable := screeningController.readID(ginContext)
	if !idIsReadable {
		return
	}

	runDto, err := screeningController.screeningApplication.CancelRun(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	if err != nil {
		screeningController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, runDto)
}

func (screeningController *SymbolScreeningController) readID(ginContext *gin.Context) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param("id"), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

// respondWithError answers a missing and a foreign profile or run the same way, so identifiers cannot be probed.
func (screeningController *SymbolScreeningController) respondWithError(ginContext *gin.Context, err error) {
	switch {
	case errors.Is(err, domains.ErrSymbolScreeningProfileNotFound), errors.Is(err, domains.ErrSymbolScreeningRunNotFound):
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
	case errors.Is(err, domains.ErrStrategyScriptNotFound):
		ginContext.JSON(http.StatusNotFound, gin.H{"message": "找不到這支策略腳本"})
	case errors.Is(err, domains.ErrSymbolScreeningProfileNameTaken), errors.Is(err, domains.ErrSymbolScreeningProfileInUse),
		errors.Is(err, domains.ErrSymbolScreeningRunAlreadyEnded):
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
	case errors.Is(err, domains.ErrSymbolScreeningRunAlreadyActive):
		ginContext.JSON(http.StatusTooManyRequests, gin.H{"message": err.Error()})
	case errors.Is(err, domains.ErrSymbolScreeningValidation), errors.Is(err, domains.ErrStrategyScriptNotYours):
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
	default:
		ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
	}
}

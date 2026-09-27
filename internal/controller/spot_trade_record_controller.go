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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type SpotTradeRecordController struct {
	spotTradeJournalApplication        *application.SpotTradeJournalApplication
	spotTradeLiveComparisonApplication *application.SpotTradeLiveComparisonApplication
}

func NewSpotTradeRecordController(
	spotTradeJournalApplication *application.SpotTradeJournalApplication,
	spotTradeLiveComparisonApplication *application.SpotTradeLiveComparisonApplication,
) *SpotTradeRecordController {
	return &SpotTradeRecordController{
		spotTradeJournalApplication:        spotTradeJournalApplication,
		spotTradeLiveComparisonApplication: spotTradeLiveComparisonApplication,
	}
}

func (recordController *SpotTradeRecordController) RecordTrade(ginContext *gin.Context) {
	var recordRequest models.SpotTradeRecordRequest
	if bindError := ginContext.ShouldBindJSON(&recordRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.RecordTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), recordRequest.ToWriteDto())
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, recordDto)
}

// ListTrades reads limit leniently: an unreadable limit falls back to the default rather than failing the list.
func (recordController *SpotTradeRecordController) ListTrades(ginContext *gin.Context) {
	limit, _ := strconv.Atoi(ginContext.Query("limit"))

	pageDto, err := recordController.spotTradeJournalApplication.ListTrades(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), dto.SpotTradeListQueryDto{
			Status: ginContext.Query("status"),
			Symbol: ginContext.Query("symbol"),
			Market: ginContext.Query("market"),
			Period: ginContext.Query("period"),
			Limit:  limit,
		})
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, pageDto)
}

func (recordController *SpotTradeRecordController) GetStatistics(ginContext *gin.Context) {
	statisticsDto, err := recordController.spotTradeJournalApplication.GetStatistics(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), ginContext.Query("period"))
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, statisticsDto)
}

// PrepareJournalLink records nothing; it only reads what the link would fill in.
func (recordController *SpotTradeRecordController) PrepareJournalLink(ginContext *gin.Context) {
	prefillDto, err := recordController.spotTradeJournalApplication.PrepareJournalLink(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), ginContext.Param("identifier"))
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, prefillDto)
}

func (recordController *SpotTradeRecordController) GetTrade(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.GetTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) DeleteTrade(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	if err := recordController.spotTradeJournalApplication.DeleteTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

func (recordController *SpotTradeRecordController) AddFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var fillRequest models.SpotTradeFillRequest
	if bindError := ginContext.ShouldBindJSON(&fillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.AddFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) AmendFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}
	fillID, fillIDIsReadable := recordController.readID(ginContext, "fillId")
	if !fillIDIsReadable {
		return
	}

	var fillRequest models.SpotTradeFillRequest
	if bindError := ginContext.ShouldBindJSON(&fillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.AmendFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillID, fillRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) RemoveFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}
	fillID, fillIDIsReadable := recordController.readID(ginContext, "fillId")
	if !fillIDIsReadable {
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.RemoveFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillID)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) AmendPlan(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var planRequest models.SpotTradePlanRequest
	if bindError := ginContext.ShouldBindJSON(&planRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.AmendPlan(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, planRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) AddNote(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var noteRequest models.SpotTradeNoteRequest
	if bindError := ginContext.ShouldBindJSON(&noteRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.AddNote(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, noteRequest.Content)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) WriteReview(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var reviewRequest models.SpotTradeReviewRequest
	if bindError := ginContext.ShouldBindJSON(&reviewRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.WriteReview(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, reviewRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) AssignSetupTags(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var setupTagsRequest models.SpotTradeSetupTagsRequest
	if bindError := ginContext.ShouldBindJSON(&setupTagsRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.spotTradeJournalApplication.AssignSetupTags(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, setupTagsRequest.SetupTagIDs)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *SpotTradeRecordController) CompareWithBacktest(ginContext *gin.Context) {
	tradingStrategyID, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	comparisonDto, err := recordController.spotTradeLiveComparisonApplication.CompareWithBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), tradingStrategyID)
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, comparisonDto)
}

func (recordController *SpotTradeRecordController) respondWithTrade(
	ginContext *gin.Context, recordDto dto.SpotTradeRecordDto, err error,
) {
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, recordDto)
}

// readID answers a bad request itself for zero, unreadable or wider-than-column identifiers; false means the response was already sent.
func (recordController *SpotTradeRecordController) readID(ginContext *gin.Context, name string) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param(name), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

func (recordController *SpotTradeRecordController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrSpotTradeValidation) || errors.Is(err, domains.ErrSpotTradeLocked) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrSpotTradeNotFound) || errors.Is(err, domains.ErrTradeTagNotFound) ||
		errors.Is(err, domains.ErrTradingStrategyNotFound) || errors.Is(err, domains.ErrJournalLinkNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if openHoldingError, isOpenHolding := errors.AsType[domains.SpotTradeOpenHoldingExistsError](err); isOpenHolding {
		body := gin.H{"message": err.Error()}
		if openHoldingError.OpenTradeID != 0 {
			body["openTradeId"] = openHoldingError.OpenTradeID
		}
		ginContext.JSON(http.StatusConflict, body)
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

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

type ContractTradeRecordController struct {
	contractTradeJournalApplication        *application.ContractTradeJournalApplication
	contractTradeLiveComparisonApplication *application.ContractTradeLiveComparisonApplication
}

func NewContractTradeRecordController(
	contractTradeJournalApplication *application.ContractTradeJournalApplication,
	contractTradeLiveComparisonApplication *application.ContractTradeLiveComparisonApplication,
) *ContractTradeRecordController {
	return &ContractTradeRecordController{
		contractTradeJournalApplication:        contractTradeJournalApplication,
		contractTradeLiveComparisonApplication: contractTradeLiveComparisonApplication,
	}
}

func (recordController *ContractTradeRecordController) RecordTrade(ginContext *gin.Context) {
	var recordRequest models.ContractTradeRecordRequest
	if bindError := ginContext.ShouldBindJSON(&recordRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.RecordTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), recordRequest.ToWriteDto())
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, recordDto)
}

// ListTrades reads limit leniently: an unreadable limit falls back to the default rather than failing the list.
func (recordController *ContractTradeRecordController) ListTrades(ginContext *gin.Context) {
	limit, _ := strconv.Atoi(ginContext.Query("limit"))

	pageDto, err := recordController.contractTradeJournalApplication.ListTrades(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), dto.ContractTradeListQueryDto{
			Status: ginContext.Query("status"),
			Symbol: ginContext.Query("symbol"),
			Period: ginContext.Query("period"),
			Limit:  limit,
		})
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, pageDto)
}

func (recordController *ContractTradeRecordController) GetStatistics(ginContext *gin.Context) {
	statisticsDto, err := recordController.contractTradeJournalApplication.GetStatistics(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), ginContext.Query("period"))
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, statisticsDto)
}

// PrepareJournalLink records nothing; it only reads what the link would fill in.
func (recordController *ContractTradeRecordController) PrepareJournalLink(ginContext *gin.Context) {
	prefillDto, err := recordController.contractTradeJournalApplication.PrepareJournalLink(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), ginContext.Param("identifier"))
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, prefillDto)
}

func (recordController *ContractTradeRecordController) GetTrade(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.GetTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) DeleteTrade(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	if err := recordController.contractTradeJournalApplication.DeleteTrade(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id); err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

func (recordController *ContractTradeRecordController) AddFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var fillRequest models.ContractTradeFillRequest
	if bindError := ginContext.ShouldBindJSON(&fillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.AddFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) AmendFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}
	fillID, fillIDIsReadable := recordController.readID(ginContext, "fillId")
	if !fillIDIsReadable {
		return
	}

	var fillRequest models.ContractTradeFillRequest
	if bindError := ginContext.ShouldBindJSON(&fillRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.AmendFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillID, fillRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) RemoveFill(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}
	fillID, fillIDIsReadable := recordController.readID(ginContext, "fillId")
	if !fillIDIsReadable {
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.RemoveFill(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, fillID)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) AmendPlan(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var planRequest models.ContractTradePlanRequest
	if bindError := ginContext.ShouldBindJSON(&planRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.AmendPlan(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, planRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) AddNote(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var noteRequest models.ContractTradeNoteRequest
	if bindError := ginContext.ShouldBindJSON(&noteRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.AddNote(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, noteRequest.Content)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) WriteReview(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var reviewRequest models.ContractTradeReviewRequest
	if bindError := ginContext.ShouldBindJSON(&reviewRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.WriteReview(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, reviewRequest.ToWriteDto())
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) AssignSetupTags(ginContext *gin.Context) {
	id, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	var setupTagsRequest models.ContractTradeSetupTagsRequest
	if bindError := ginContext.ShouldBindJSON(&setupTagsRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	recordDto, err := recordController.contractTradeJournalApplication.AssignSetupTags(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), id, setupTagsRequest.SetupTagIDs)
	recordController.respondWithTrade(ginContext, recordDto, err)
}

func (recordController *ContractTradeRecordController) CompareWithBacktest(ginContext *gin.Context) {
	tradingStrategyID, idIsReadable := recordController.readID(ginContext, "id")
	if !idIsReadable {
		return
	}

	comparisonDto, err := recordController.contractTradeLiveComparisonApplication.CompareWithBacktest(
		ginContext.Request.Context(), middlewares.CurrentUserID(ginContext), tradingStrategyID)
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, comparisonDto)
}

func (recordController *ContractTradeRecordController) respondWithTrade(
	ginContext *gin.Context, recordDto dto.ContractTradeRecordDto, err error,
) {
	if err != nil {
		recordController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, recordDto)
}

// readID answers a bad request itself for zero, unreadable or wider-than-column identifiers; false means the response was already sent.
func (recordController *ContractTradeRecordController) readID(ginContext *gin.Context, name string) (uint, bool) {
	id, parseError := strconv.ParseUint(ginContext.Param(name), 10, strconv.IntSize)
	if parseError != nil || id == 0 || id > math.MaxInt64 {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "識別碼必須是正整數"})
		return 0, false
	}

	return uint(id), true
}

func (recordController *ContractTradeRecordController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrContractTradeValidation) || errors.Is(err, domains.ErrContractTradeLocked) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrContractTradeNotFound) || errors.Is(err, domains.ErrTradeTagNotFound) ||
		errors.Is(err, domains.ErrTradingStrategyNotFound) || errors.Is(err, domains.ErrJournalLinkNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrContractTradeOpenPositionExists) {
		ginContext.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

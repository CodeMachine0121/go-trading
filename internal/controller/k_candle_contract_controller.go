package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

type KCandleContractController struct {
	kCandleContractApplication *application.KCandleContractApplication
}

func NewKCandleContractController(
	kCandleContractApplication *application.KCandleContractApplication,
) *KCandleContractController {
	return &KCandleContractController{kCandleContractApplication: kCandleContractApplication}
}

// CreateKCandleContract handles POST /contract-k-candles.
func (kCandleContractController *KCandleContractController) CreateKCandleContract(
	ginContext *gin.Context,
) {
	var contractRequest models.KCandleContractRequest

	if bindError := ginContext.ShouldBindJSON(&contractRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})

		return
	}

	contractDto, saveError := kCandleContractController.kCandleContractApplication.
		SaveKCandleContract(ginContext.Request.Context(),
			contractRequest.ToWriteDto(contractRequest.Symbol, contractRequest.OpenTime))
	if saveError != nil {
		kCandleContractController.respondWithError(ginContext, saveError)

		return
	}

	ginContext.JSON(http.StatusOK, contractDto)
}

// GetKCandleContractsInRange handles GET /contract-k-candles.
func (kCandleContractController *KCandleContractController) GetKCandleContractsInRange(
	ginContext *gin.Context,
) {
	startTime, startTimeIsReadable := kCandleContractController.readTime(
		ginContext, "startTime", ginContext.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleContractController.readTime(
		ginContext, "endTime", ginContext.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	contractDtos, findError := kCandleContractController.kCandleContractApplication.
		GetKCandleContractsInRange(ginContext.Request.Context(), dto.KCandleQueryDto{
			Symbol:    ginContext.Query("symbol"),
			StartTime: startTime,
			EndTime:   endTime,
		})
	if findError != nil {
		kCandleContractController.respondWithError(ginContext, findError)

		return
	}

	ginContext.JSON(http.StatusOK, contractDtos)
}

// GetKCandleContractSeries handles GET /contract-k-candles/series with the same parameters as the spot series.
func (kCandleContractController *KCandleContractController) GetKCandleContractSeries(
	ginContext *gin.Context,
) {
	startTime, startTimeIsReadable := kCandleContractController.readTime(
		ginContext, "startTime", ginContext.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleContractController.readTime(
		ginContext, "endTime", ginContext.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	displayableCandleCount := (*int)(nil)
	if declaredCount := ginContext.Query("displayableCandleCount"); declaredCount != "" {
		parsedCount, parseError := strconv.Atoi(declaredCount)
		if parseError != nil {
			ginContext.JSON(http.StatusBadRequest, gin.H{
				"message": "displayableCandleCount 必須是整數",
				"field":   "displayableCandleCount",
			})

			return
		}
		displayableCandleCount = &parsedCount
	}

	seriesDto, findError := kCandleContractController.kCandleContractApplication.
		GetKCandleContractSeries(ginContext.Request.Context(), dto.KCandleSeriesQueryDto{
			Symbol:                 ginContext.Query("symbol"),
			StartTime:              startTime,
			EndTime:                endTime,
			Interval:               ginContext.Query("interval"),
			DisplayableCandleCount: displayableCandleCount,
		})
	if findError != nil {
		kCandleContractController.respondWithError(ginContext, findError)

		return
	}

	ginContext.JSON(http.StatusOK, seriesDto)
}

// GetKCandleContract handles GET /contract-k-candles/:symbol/:openTime.
func (kCandleContractController *KCandleContractController) GetKCandleContract(
	ginContext *gin.Context,
) {
	openTime, openTimeIsReadable := kCandleContractController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	contractDto, findError := kCandleContractController.kCandleContractApplication.
		GetKCandleContract(ginContext.Request.Context(), ginContext.Param("symbol"), openTime)
	if findError != nil {
		kCandleContractController.respondWithError(ginContext, findError)

		return
	}

	ginContext.JSON(http.StatusOK, contractDto)
}

// UpdateKCandleContract handles PUT /contract-k-candles/:symbol/:openTime.
func (kCandleContractController *KCandleContractController) UpdateKCandleContract(
	ginContext *gin.Context,
) {
	symbol := ginContext.Param("symbol")

	openTime, openTimeIsReadable := kCandleContractController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	var contractRequest models.KCandleContractRequest

	if bindError := ginContext.ShouldBindJSON(&contractRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})

		return
	}

	movesSymbol := contractRequest.Symbol != "" && contractRequest.Symbol != symbol
	movesOpenTime := !contractRequest.OpenTime.IsZero() && !contractRequest.OpenTime.Equal(openTime)
	if movesSymbol || movesOpenTime {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "不得更換交易標的與起始時間"})

		return
	}

	contractDto, updateError := kCandleContractController.kCandleContractApplication.
		UpdateKCandleContract(ginContext.Request.Context(),
			contractRequest.ToWriteDto(symbol, openTime))
	if updateError != nil {
		kCandleContractController.respondWithError(ginContext, updateError)

		return
	}

	ginContext.JSON(http.StatusOK, contractDto)
}

// DeleteKCandleContract handles DELETE /contract-k-candles/:symbol/:openTime; the same-named spot candle is untouched.
func (kCandleContractController *KCandleContractController) DeleteKCandleContract(
	ginContext *gin.Context,
) {
	openTime, openTimeIsReadable := kCandleContractController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	deleteError := kCandleContractController.kCandleContractApplication.DeleteKCandleContract(
		ginContext.Request.Context(), ginContext.Param("symbol"), openTime)
	if deleteError != nil {
		kCandleContractController.respondWithError(ginContext, deleteError)

		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readTime answers a bad request itself when the RFC3339 time is unreadable; false means the response was already sent.
func (kCandleContractController *KCandleContractController) readTime(
	ginContext *gin.Context, name string, value string,
) (time.Time, bool) {
	parsedTime, parseError := time.Parse(time.RFC3339, value)
	if parseError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": name + " 必須為 RFC3339 格式的時間"})

		return time.Time{}, false
	}

	return parsedTime, true
}

func (kCandleContractController *KCandleContractController) respondWithError(
	ginContext *gin.Context, respondedError error,
) {
	if errors.Is(respondedError, domains.ErrKCandleContractValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": respondedError.Error()})

		return
	}
	if errors.Is(respondedError, domains.ErrKCandleContractNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": respondedError.Error()})

		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": respondedError.Error()})
}

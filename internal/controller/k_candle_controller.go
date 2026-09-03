package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// KCandleController exposes the K candle use cases over HTTP.
type KCandleController struct {
	kCandleApplication *application.KCandleApplication
}

func NewKCandleController(kCandleApplication *application.KCandleApplication) *KCandleController {
	return &KCandleController{kCandleApplication: kCandleApplication}
}

// CreateKCandle handles POST /k-candles.
func (kCandleController *KCandleController) CreateKCandle(ginContext *gin.Context) {
	var kCandleRequest models.KCandleRequest

	if bindError := ginContext.ShouldBindJSON(&kCandleRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.SaveKCandle(ginContext.Request.Context(),
		kCandleRequest.ToWriteDto(kCandleRequest.Symbol, kCandleRequest.OpenTime))
	if err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, kCandleDto)
}

// GetKCandlesInRange handles GET /k-candles.
func (kCandleController *KCandleController) GetKCandlesInRange(ginContext *gin.Context) {
	startTime, startTimeIsReadable := kCandleController.readTime(
		ginContext, "startTime", ginContext.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleController.readTime(ginContext, "endTime", ginContext.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	kCandleDtos, err := kCandleController.kCandleApplication.GetKCandlesInRange(ginContext.Request.Context(), dto.KCandleQueryDto{
		Symbol:    ginContext.Query("symbol"),
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, kCandleDtos)
}

// GetKCandleSeries handles GET /k-candles/series.
func (kCandleController *KCandleController) GetKCandleSeries(ginContext *gin.Context) {
	startTime, startTimeIsReadable := kCandleController.readTime(
		ginContext, "startTime", ginContext.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleController.readTime(ginContext, "endTime", ginContext.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	kCandleSeriesDto, err := kCandleController.kCandleApplication.GetKCandleSeries(ginContext.Request.Context(), dto.KCandleSeriesQueryDto{
		Symbol:    ginContext.Query("symbol"),
		StartTime: startTime,
		EndTime:   endTime,
		Interval:  ginContext.Query("interval"),
	})
	if err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, kCandleSeriesDto)
}

// GetKCandle handles GET /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) GetKCandle(ginContext *gin.Context) {
	openTime, openTimeIsReadable := kCandleController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.GetKCandle(ginContext.Request.Context(), ginContext.Param("symbol"), openTime)
	if err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, kCandleDto)
}

// UpdateKCandle handles PUT /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) UpdateKCandle(ginContext *gin.Context) {
	symbol := ginContext.Param("symbol")

	openTime, openTimeIsReadable := kCandleController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	var kCandleRequest models.KCandleRequest

	if bindError := ginContext.ShouldBindJSON(&kCandleRequest); bindError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	movesSymbol := kCandleRequest.Symbol != "" && kCandleRequest.Symbol != symbol
	movesOpenTime := !kCandleRequest.OpenTime.IsZero() && !kCandleRequest.OpenTime.Equal(openTime)
	if movesSymbol || movesOpenTime {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "不得更換交易標的與起始時間"})
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.UpdateKCandle(ginContext.Request.Context(),
		kCandleRequest.ToWriteDto(symbol, openTime))
	if err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, kCandleDto)
}

// DeleteKCandle handles DELETE /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) DeleteKCandle(ginContext *gin.Context) {
	openTime, openTimeIsReadable := kCandleController.readTime(
		ginContext, "openTime", ginContext.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	if err := kCandleController.kCandleApplication.DeleteKCandle(ginContext.Request.Context(), ginContext.Param("symbol"), openTime); err != nil {
		kCandleController.respondWithError(ginContext, err)
		return
	}

	ginContext.Status(http.StatusNoContent)
}

// readTime reads one RFC3339 time out of the request, answering the caller with a
// bad request when it cannot be read. The second return value says whether the
// handler may carry on — a handler that gets false has already had its answer sent.
func (kCandleController *KCandleController) readTime(
	ginContext *gin.Context, name string, value string,
) (time.Time, bool) {
	parsedTime, parseError := time.Parse(time.RFC3339, value)
	if parseError != nil {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": name + " 必須為 RFC3339 格式的時間"})
		return time.Time{}, false
	}

	return parsedTime, true
}

// respondWithError maps a domain error onto the status code that reports it.
func (kCandleController *KCandleController) respondWithError(ginContext *gin.Context, err error) {
	if errors.Is(err, domains.ErrKCandleValidation) {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrKCandleNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	ginContext.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

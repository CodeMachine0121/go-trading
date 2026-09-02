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
func (kCandleController *KCandleController) CreateKCandle(context *gin.Context) {
	var kCandleRequest models.KCandleRequest

	if bindError := context.ShouldBindJSON(&kCandleRequest); bindError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.SaveKCandle(
		kCandleRequest.ToWriteDto(kCandleRequest.Symbol, kCandleRequest.OpenTime))
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, kCandleDto)
}

// GetKCandlesInRange handles GET /k-candles.
func (kCandleController *KCandleController) GetKCandlesInRange(context *gin.Context) {
	startTime, startTimeIsReadable := kCandleController.readTime(
		context, "startTime", context.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleController.readTime(context, "endTime", context.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	kCandleDtos, err := kCandleController.kCandleApplication.GetKCandlesInRange(dto.KCandleQueryDto{
		Symbol:    context.Query("symbol"),
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, kCandleDtos)
}

// GetKCandleSeries handles GET /k-candles/series.
func (kCandleController *KCandleController) GetKCandleSeries(context *gin.Context) {
	startTime, startTimeIsReadable := kCandleController.readTime(
		context, "startTime", context.Query("startTime"))
	if !startTimeIsReadable {
		return
	}

	endTime, endTimeIsReadable := kCandleController.readTime(context, "endTime", context.Query("endTime"))
	if !endTimeIsReadable {
		return
	}

	kCandleSeriesDto, err := kCandleController.kCandleApplication.GetKCandleSeries(dto.KCandleSeriesQueryDto{
		Symbol:    context.Query("symbol"),
		StartTime: startTime,
		EndTime:   endTime,
		Interval:  context.Query("interval"),
	})
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, kCandleSeriesDto)
}

// ListTradingSymbols handles GET /trading-symbols.
func (kCandleController *KCandleController) ListTradingSymbols(context *gin.Context) {
	tradingSymbolDtos, err := kCandleController.kCandleApplication.ListTradingSymbols()
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, tradingSymbolDtos)
}

// GetKCandle handles GET /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) GetKCandle(context *gin.Context) {
	openTime, openTimeIsReadable := kCandleController.readTime(
		context, "openTime", context.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.GetKCandle(context.Param("symbol"), openTime)
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, kCandleDto)
}

// UpdateKCandle handles PUT /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) UpdateKCandle(context *gin.Context) {
	symbol := context.Param("symbol")

	openTime, openTimeIsReadable := kCandleController.readTime(
		context, "openTime", context.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	var kCandleRequest models.KCandleRequest

	if bindError := context.ShouldBindJSON(&kCandleRequest); bindError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": bindError.Error()})
		return
	}

	movesSymbol := kCandleRequest.Symbol != "" && kCandleRequest.Symbol != symbol
	movesOpenTime := !kCandleRequest.OpenTime.IsZero() && !kCandleRequest.OpenTime.Equal(openTime)
	if movesSymbol || movesOpenTime {
		context.JSON(http.StatusBadRequest, gin.H{"message": "不得更換交易標的與起始時間"})
		return
	}

	kCandleDto, err := kCandleController.kCandleApplication.UpdateKCandle(
		kCandleRequest.ToWriteDto(symbol, openTime))
	if err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.JSON(http.StatusOK, kCandleDto)
}

// DeleteKCandle handles DELETE /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) DeleteKCandle(context *gin.Context) {
	openTime, openTimeIsReadable := kCandleController.readTime(
		context, "openTime", context.Param("openTime"))
	if !openTimeIsReadable {
		return
	}

	if err := kCandleController.kCandleApplication.DeleteKCandle(context.Param("symbol"), openTime); err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.Status(http.StatusNoContent)
}

// readTime reads one RFC3339 time out of the request, answering the caller with a
// bad request when it cannot be read. The second return value says whether the
// handler may carry on — a handler that gets false has already had its answer sent.
func (kCandleController *KCandleController) readTime(
	context *gin.Context, name string, value string,
) (time.Time, bool) {
	parsedTime, parseError := time.Parse(time.RFC3339, value)
	if parseError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": name + " 必須為 RFC3339 格式的時間"})
		return time.Time{}, false
	}

	return parsedTime, true
}

// respondWithError maps a domain error onto the status code that reports it.
func (kCandleController *KCandleController) respondWithError(context *gin.Context, err error) {
	if errors.Is(err, domains.ErrKCandleValidation) {
		context.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if errors.Is(err, domains.ErrKCandleNotFound) {
		context.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	context.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
}

package controller

import (
	"errors"
	"net/http"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// KCandleRequest is the body a caller sends to create or update a K candle.
// On update the candle is named by the path, so a symbol or open time in the body
// is only accepted when it matches.
type KCandleRequest struct {
	Symbol              string          `json:"symbol"`
	OpenTime            time.Time       `json:"openTime"`
	Open                decimal.Decimal `json:"open"`
	High                decimal.Decimal `json:"high"`
	Low                 decimal.Decimal `json:"low"`
	Close               decimal.Decimal `json:"close"`
	Volume              decimal.Decimal `json:"volume"`
	QuoteVolume         decimal.Decimal `json:"quoteVolume"`
	TakerBuyBaseVolume  decimal.Decimal `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.Decimal `json:"takerBuyQuoteVolume"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking the
// identity from the arguments so the caller of this method decides what is named.
func (kCandleRequest KCandleRequest) ToWriteDto(symbol string, openTime time.Time) dto.KCandleWriteDto {
	return dto.KCandleWriteDto{
		Symbol:              symbol,
		OpenTime:            openTime,
		Open:                kCandleRequest.Open,
		High:                kCandleRequest.High,
		Low:                 kCandleRequest.Low,
		Close:               kCandleRequest.Close,
		Volume:              kCandleRequest.Volume,
		QuoteVolume:         kCandleRequest.QuoteVolume,
		TakerBuyBaseVolume:  kCandleRequest.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: kCandleRequest.TakerBuyQuoteVolume,
	}
}

// KCandleController exposes the K candle use cases over HTTP.
type KCandleController struct {
	kCandleApplication *application.KCandleApplication
}

func NewKCandleController(kCandleApplication *application.KCandleApplication) *KCandleController {
	return &KCandleController{kCandleApplication: kCandleApplication}
}

// CreateKCandle handles POST /k-candles.
func (kCandleController *KCandleController) CreateKCandle(context *gin.Context) {
	var kCandleRequest KCandleRequest

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
	startTime, startTimeError := time.Parse(time.RFC3339, context.Query("startTime"))
	if startTimeError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "startTime 必須為 RFC3339 格式的時間"})
		return
	}

	endTime, endTimeError := time.Parse(time.RFC3339, context.Query("endTime"))
	if endTimeError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "endTime 必須為 RFC3339 格式的時間"})
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

// GetKCandle handles GET /k-candles/:symbol/:openTime.
func (kCandleController *KCandleController) GetKCandle(context *gin.Context) {
	openTime, openTimeError := time.Parse(time.RFC3339, context.Param("openTime"))
	if openTimeError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "openTime 必須為 RFC3339 格式的時間"})
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

	openTime, openTimeError := time.Parse(time.RFC3339, context.Param("openTime"))
	if openTimeError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "openTime 必須為 RFC3339 格式的時間"})
		return
	}

	var kCandleRequest KCandleRequest

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
	openTime, openTimeError := time.Parse(time.RFC3339, context.Param("openTime"))
	if openTimeError != nil {
		context.JSON(http.StatusBadRequest, gin.H{"message": "openTime 必須為 RFC3339 格式的時間"})
		return
	}

	if err := kCandleController.kCandleApplication.DeleteKCandle(context.Param("symbol"), openTime); err != nil {
		kCandleController.respondWithError(context, err)
		return
	}

	context.Status(http.StatusNoContent)
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

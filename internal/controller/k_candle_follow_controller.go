package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// KCandleFollowController streams live spot and contract K candle updates as server-sent events, only translating what the application hands it.
type KCandleFollowController struct {
	kCandleFollowApplication         *application.KCandleFollowApplication
	kCandleContractFollowApplication *application.KCandleContractFollowApplication
}

func NewKCandleFollowController(
	kCandleFollowApplication *application.KCandleFollowApplication,
	kCandleContractFollowApplication *application.KCandleContractFollowApplication,
) *KCandleFollowController {
	return &KCandleFollowController{
		kCandleFollowApplication:         kCandleFollowApplication,
		kCandleContractFollowApplication: kCandleContractFollowApplication,
	}
}

// WatchKCandles handles GET /k-candles/live?symbol=BTCUSDT.
func (kCandleFollowController *KCandleFollowController) WatchKCandles(ginContext *gin.Context) {
	symbol := ginContext.Query("symbol")
	if symbol == "" {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定交易標的"})

		return
	}

	updates, watchError := kCandleFollowController.kCandleFollowApplication.
		WatchKCandles(ginContext.Request.Context(), symbol)

	kCandleFollowController.stream(ginContext, updates, watchError)
}

// WatchKCandleContracts handles GET /contract-k-candles/live?symbol=BTCUSDT, never following the spot market of the same code.
func (kCandleFollowController *KCandleFollowController) WatchKCandleContracts(ginContext *gin.Context) {
	symbol := ginContext.Query("symbol")
	if symbol == "" {
		ginContext.JSON(http.StatusBadRequest, gin.H{"message": "請指定合約標的"})

		return
	}

	updates, watchError := kCandleFollowController.kCandleContractFollowApplication.
		WatchKCandleContracts(ginContext.Request.Context(), symbol)

	kCandleFollowController.stream(ginContext, updates, watchError)
}

// stream answers a refused follow with the matching status, otherwise writes each update as an SSE event until the updates or the request end.
func (kCandleFollowController *KCandleFollowController) stream(
	ginContext *gin.Context, updates <-chan dto.KCandleFollowUpdateDto, watchError error,
) {
	if watchError != nil {
		// An unknown market is the caller's to fix and must not read like a system shutting down.
		switch {
		case errors.Is(watchError, domains.ErrTradingSymbolNotRegistered):
			ginContext.JSON(http.StatusNotFound, gin.H{"message": watchError.Error()})
		case errors.Is(watchError, domains.ErrKCandleContractValidation):
			ginContext.JSON(http.StatusBadRequest, gin.H{"message": watchError.Error()})
		case errors.Is(watchError, domains.ErrContractTradingSymbolNotWatched):
			ginContext.JSON(http.StatusConflict, gin.H{"message": watchError.Error()})
		default:
			ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": watchError.Error()})
		}

		return
	}

	ginContext.Header("Content-Type", "text/event-stream")
	ginContext.Header("Cache-Control", "no-cache")
	ginContext.Header("Connection", "keep-alive")
	// Stops buffering proxies from holding updates until the connection ends.
	ginContext.Header("X-Accel-Buffering", "no")

	// Written by hand because gin's streaming helper relies on the deprecated CloseNotify; the request context covers every way a request ends.
	requestEnded := ginContext.Request.Context().Done()
	for {
		select {
		case <-requestEnded:
			return

		case update, isDelivering := <-updates:
			if !isDelivering {
				return
			}

			body, encodeError := json.Marshal(update)
			if encodeError != nil {
				return
			}

			if _, writeError := fmt.Fprintf(ginContext.Writer, "data: %s\n\n", body); writeError != nil {
				return
			}
			ginContext.Writer.Flush()
		}
	}
}

package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/gin-gonic/gin"
)

// KCandleFollowController streams live K candle updates to one viewer.
//
// It is the only place that knows how an update reaches a browser, and it does
// nothing but translate: it writes whatever it is handed, one event per update, and
// leaves when the updates end or the request does. Every decision about what to
// send and when was already made before it was handed the channel, which is why a
// connection that lives for hours is still only doing request-to-response
// translation — just repeatedly.
type KCandleFollowController struct {
	kCandleFollowApplication *application.KCandleFollowApplication
}

func NewKCandleFollowController(
	kCandleFollowApplication *application.KCandleFollowApplication,
) *KCandleFollowController {
	return &KCandleFollowController{kCandleFollowApplication: kCandleFollowApplication}
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
	// Starting to watch fails for exactly one reason today — the system is shutting
	// down — so there is one answer. A second reason will need a second answer, and
	// writing that answer now would only be a guess at what it should be.
	if watchError != nil {
		ginContext.JSON(http.StatusServiceUnavailable, gin.H{"message": watchError.Error()})

		return
	}

	ginContext.Header("Content-Type", "text/event-stream")
	ginContext.Header("Cache-Control", "no-cache")
	ginContext.Header("Connection", "keep-alive")
	// Proxies that buffer would hold updates back until the connection ends, which
	// is the one thing a live feed cannot survive.
	ginContext.Header("X-Accel-Buffering", "no")

	// Written by hand rather than through the framework's streaming helper, because
	// that helper watches a connection-closed signal the standard library has
	// deprecated. The request's own context says the same thing, and says it for
	// every way a request can end rather than only for a dropped connection.
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
			// Without this the update sits in a buffer until the connection ends,
			// which for a feed that never ends means it is never seen.
			ginContext.Writer.Flush()
		}
	}
}

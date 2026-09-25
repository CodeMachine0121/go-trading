package middlewares

import (
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/gin-gonic/gin"
)

// LiveStreamLimitMiddleware holds a live stream place for as long as the stream's handler runs.
type LiveStreamLimitMiddleware struct {
	requestAdmissionApplication *application.RequestAdmissionApplication
}

func NewLiveStreamLimitMiddleware(
	requestAdmissionApplication *application.RequestAdmissionApplication,
) *LiveStreamLimitMiddleware {
	return &LiveStreamLimitMiddleware{requestAdmissionApplication: requestAdmissionApplication}
}

func (liveStreamLimitMiddleware *LiveStreamLimitMiddleware) Handle(ginContext *gin.Context) {
	liveStreamSlot, openError := liveStreamLimitMiddleware.requestAdmissionApplication.OpenLiveStream(
		requesterOf(ginContext))
	if openError != nil {
		ginContext.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": openError.Error()})
		return
	}
	defer liveStreamLimitMiddleware.requestAdmissionApplication.CloseLiveStream(liveStreamSlot)

	ginContext.Next()
}

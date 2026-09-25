package middlewares

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequestBodyLimitMiddleware refuses a declared oversize body outright and cuts an undeclared one off at the limit.
type RequestBodyLimitMiddleware struct {
	maximumBytes int64
}

func NewRequestBodyLimitMiddleware(maximumBytes int64) *RequestBodyLimitMiddleware {
	return &RequestBodyLimitMiddleware{maximumBytes: maximumBytes}
}

func (requestBodyLimitMiddleware *RequestBodyLimitMiddleware) Handle(ginContext *gin.Context) {
	if ginContext.Request.ContentLength > requestBodyLimitMiddleware.maximumBytes {
		ginContext.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
			"message": fmt.Sprintf("送入的內容太大，上限為 %d KB", requestBodyLimitMiddleware.maximumBytes>>10),
		})
		return
	}

	ginContext.Request.Body = http.MaxBytesReader(
		ginContext.Writer, ginContext.Request.Body, requestBodyLimitMiddleware.maximumBytes)
	ginContext.Next()
}

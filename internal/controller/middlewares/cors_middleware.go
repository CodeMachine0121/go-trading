package middlewares

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// CorsMiddleware adds permission headers only for the configured origins, so browsers refuse responses to any other origin.
type CorsMiddleware struct {
	allowedOrigins []string
}

func NewCorsMiddleware(allowedOrigins []string) *CorsMiddleware {
	return &CorsMiddleware{allowedOrigins: allowedOrigins}
}

// Handle ends preflight OPTIONS requests here instead of routing them.
func (corsMiddleware *CorsMiddleware) Handle(ginContext *gin.Context) {
	ginContext.Header("Vary", "Origin")

	requestOrigin := ginContext.GetHeader("Origin")
	if slices.Contains(corsMiddleware.allowedOrigins, requestOrigin) {
		ginContext.Header("Access-Control-Allow-Origin", requestOrigin)
		ginContext.Header("Access-Control-Allow-Methods", strings.Join(allowedCorsMethods, ", "))
		ginContext.Header("Access-Control-Allow-Headers", strings.Join(allowedCorsHeaders, ", "))
		ginContext.Header("Access-Control-Max-Age", corsPreflightMaxAgeSeconds)
	}

	if ginContext.Request.Method == http.MethodOptions {
		ginContext.AbortWithStatus(http.StatusNoContent)
		return
	}

	ginContext.Next()
}

var (
	allowedCorsMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}
	// Browsers will not send a bearer token unless Authorization is allowed.
	allowedCorsHeaders = []string{"Content-Type", "Authorization"}
)

const corsPreflightMaxAgeSeconds = "600"

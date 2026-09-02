package controller

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// CorsMiddleware lets a browser front end on another origin talk to this API. Only
// the origins it was built with are answered; anything else is served without the
// permission headers, which is how the browser refuses the response on our behalf.
type CorsMiddleware struct {
	allowedOrigins []string
}

func NewCorsMiddleware(allowedOrigins []string) *CorsMiddleware {
	return &CorsMiddleware{allowedOrigins: allowedOrigins}
}

// Handle answers the browser's permission questions. A preflight OPTIONS is a
// question only, never a use case, so it ends here instead of reaching a route.
func (corsMiddleware *CorsMiddleware) Handle(context *gin.Context) {
	context.Header("Vary", "Origin")

	requestOrigin := context.GetHeader("Origin")
	if slices.Contains(corsMiddleware.allowedOrigins, requestOrigin) {
		context.Header("Access-Control-Allow-Origin", requestOrigin)
		context.Header("Access-Control-Allow-Methods", strings.Join(allowedCorsMethods, ", "))
		context.Header("Access-Control-Allow-Headers", strings.Join(allowedCorsHeaders, ", "))
		context.Header("Access-Control-Max-Age", corsPreflightMaxAgeSeconds)
	}

	if context.Request.Method == http.MethodOptions {
		context.AbortWithStatus(http.StatusNoContent)
		return
	}

	context.Next()
}

var (
	allowedCorsMethods = []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}
	allowedCorsHeaders = []string{"Content-Type"}
)

const corsPreflightMaxAgeSeconds = "600"

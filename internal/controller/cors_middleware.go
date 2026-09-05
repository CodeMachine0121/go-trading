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
	// Authorization is on the list because a browser will not send a proof of
	// identity it was not given permission to send — and a front end that cannot
	// send one can only ever reach the two endpoints that need none.
	allowedCorsHeaders = []string{"Content-Type", "Authorization"}
)

const corsPreflightMaxAgeSeconds = "600"

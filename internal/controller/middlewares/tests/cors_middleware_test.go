package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCorsMiddlewareAnswersOnlyTheOriginsItWasGiven(t *testing.T) {
	testCases := []struct {
		name                  string
		requestOrigin         string
		expectedAllowedOrigin string
	}{
		{
			name:                  "the front end we named is let in",
			requestOrigin:         "http://localhost:3000",
			expectedAllowedOrigin: "http://localhost:3000",
		},
		{
			name:                  "another page gets no permission",
			requestOrigin:         "http://evil.example",
			expectedAllowedOrigin: "",
		},
		{
			name:                  "a trailing slash is not the same origin",
			requestOrigin:         "http://localhost:3000/",
			expectedAllowedOrigin: "",
		},
		{
			name:                  "a caller that is not a browser sends no origin",
			requestOrigin:         "",
			expectedAllowedOrigin: "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			if testCase.requestOrigin != "" {
				request.Header.Set("Origin", testCase.requestOrigin)
			}

			corsEngine([]string{"http://localhost:3000"}).ServeHTTP(recorder, request)

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t,
				testCase.expectedAllowedOrigin,
				recorder.Header().Get("Access-Control-Allow-Origin"))
		})
	}
}

func TestCorsMiddlewareAnswersThePreflightWithoutReachingARoute(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/k-candles", nil)
	request.Header.Set("Origin", "http://localhost:3000")

	corsEngine([]string{"http://localhost:3000"}).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNoContent, recorder.Code)
	assert.Equal(t, "http://localhost:3000", recorder.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Methods"), http.MethodPost)
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
	// Without this, browsers drop the Authorization header and a signed-in front end looks signed out.
	assert.Contains(t, recorder.Header().Get("Access-Control-Allow-Headers"), "Authorization")
	// Without this, a throttled front end cannot read how long to wait.
	assert.Contains(t, recorder.Header().Get("Access-Control-Expose-Headers"), "Retry-After")
}

func corsEngine(allowedOrigins []string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middlewares.NewCorsMiddleware(allowedOrigins).Handle)
	engine.GET("/health", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"status": "Healthy"})
	})

	return engine
}

package middlewares_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bodyLimitBytes = 1024 << 10

func TestRequestBodyLimitAcceptsUpToTheLimitAndRefusesBeyond(t *testing.T) {
	testCases := []struct {
		name              string
		bodyBytes         int
		declaredLength    bool
		expectedStatus    int
		expectedMessage   string
		expectedReadBytes int
	}{
		{name: "an ordinary script", bodyBytes: 200 << 10, declaredLength: true,
			expectedStatus: http.StatusOK, expectedReadBytes: 200 << 10},
		{name: "exactly the limit", bodyBytes: bodyLimitBytes, declaredLength: true,
			expectedStatus: http.StatusOK, expectedReadBytes: bodyLimitBytes},
		{name: "one byte over, declared", bodyBytes: bodyLimitBytes + 1, declaredLength: true,
			expectedStatus: http.StatusRequestEntityTooLarge, expectedMessage: "送入的內容太大，上限為 1024 KB"},
		{name: "over, undeclared, is cut off at the limit", bodyBytes: bodyLimitBytes + 1,
			expectedStatus: http.StatusBadRequest, expectedReadBytes: bodyLimitBytes},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			readBytes := 0
			engine := gin.New()
			engine.Use(middlewares.NewRequestBodyLimitMiddleware(bodyLimitBytes).Handle)
			engine.POST("/strategy-scripts", func(ginContext *gin.Context) {
				body, readError := io.ReadAll(ginContext.Request.Body)
				readBytes = len(body)
				if readError != nil {
					ginContext.Status(http.StatusBadRequest)
					return
				}
				ginContext.Status(http.StatusOK)
			})
			request := httptest.NewRequest(http.MethodPost, "/strategy-scripts",
				strings.NewReader(strings.Repeat("a", testCase.bodyBytes)))
			if !testCase.declaredLength {
				// Unknown length, as a chunked upload arrives.
				request.ContentLength = -1
			}

			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)

			assert.Equal(t, testCase.expectedStatus, recorder.Code)
			assert.Equal(t, testCase.expectedReadBytes, readBytes)
			if testCase.expectedMessage != "" {
				var response map[string]string
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
				assert.Equal(t, testCase.expectedMessage, response["message"])
			}
		})
	}
}

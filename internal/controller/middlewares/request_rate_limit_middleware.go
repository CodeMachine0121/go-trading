package middlewares

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// RequestRateLimitMiddleware refuses a requester who has spent their allowance with 429 and Retry-After.
type RequestRateLimitMiddleware struct {
	requestAdmissionApplication *application.RequestAdmissionApplication
}

func NewRequestRateLimitMiddleware(
	requestAdmissionApplication *application.RequestAdmissionApplication,
) *RequestRateLimitMiddleware {
	return &RequestRateLimitMiddleware{requestAdmissionApplication: requestAdmissionApplication}
}

// Handle spends the general allowance and runs for every route.
func (requestRateLimitMiddleware *RequestRateLimitMiddleware) Handle(ginContext *gin.Context) {
	requestRateLimitMiddleware.admitOrRefuse(ginContext,
		requestRateLimitMiddleware.requestAdmissionApplication.AdmitRequest(requesterOf(ginContext)))
}

// HandleCredentialRequest spends the stricter allowance and is mounted only on the routes that check a password or session.
func (requestRateLimitMiddleware *RequestRateLimitMiddleware) HandleCredentialRequest(ginContext *gin.Context) {
	requestRateLimitMiddleware.admitOrRefuse(ginContext,
		requestRateLimitMiddleware.requestAdmissionApplication.AdmitCredentialRequest(requesterOf(ginContext)))
}

func (requestRateLimitMiddleware *RequestRateLimitMiddleware) admitOrRefuse(
	ginContext *gin.Context, admissionError error,
) {
	var rateExceeded domains.RequestRateExceededError
	if errors.As(admissionError, &rateExceeded) {
		ginContext.Header("Retry-After", strconv.Itoa(rateExceeded.RetryAfterSeconds()))
		ginContext.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"message": rateExceeded.Error()})
		return
	}

	ginContext.Next()
}

// requesterOf reads the address through the engine's trusted-proxy settings, never the raw forwarding headers.
func requesterOf(ginContext *gin.Context) dto.RequesterDto {
	return dto.RequesterDto{AccessToken: accessTokenOf(ginContext), ClientAddress: ginContext.ClientIP()}
}

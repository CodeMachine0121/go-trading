package main

import (
	"log"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/clock"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/gin-gonic/gin"
)

// requestGuards are the per-route limits; guardRequests has already installed the global ones.
type requestGuards struct {
	credentialRequest gin.HandlerFunc
	liveStream        gin.HandlerFunc
}

// guardRequests must run before any route is registered, because gin copies global middleware into each route
// at registration time.
func guardRequests(engine *gin.Engine, applicationConfig config.ApplicationConfig) requestGuards {
	requestLimit := applicationConfig.RequestLimit

	// Always set, because gin otherwise believes forwarding headers from anyone.
	if trustError := engine.SetTrustedProxies(requestLimit.TrustedProxyCidrs); trustError != nil {
		log.Fatalf("invalid TRUSTED_PROXY_CIDRS: %v", trustError)
	}
	engine.RemoteIPHeaders = requestLimit.ClientIpHeaders

	requestAdmissionApplication := application.NewRequestAdmissionApplication(
		service.NewRequestAdmissionService(
			security.NewJwtAccessTokenProxy(applicationConfig.Authentication.AccessTokenSigningKey),
			clock.NewSystemClockProxy(),
			vo.RequestBudgetVo{
				RequestsPerMinute: requestLimit.RequestsPerMinute,
				Burst:             requestLimit.RequestBurst,
			},
			vo.RequestBudgetVo{
				RequestsPerMinute: requestLimit.CredentialRequestsPerMinute,
				Burst:             requestLimit.CredentialRequestBurst,
			},
			vo.LiveStreamCapacityVo{
				PerRequester: requestLimit.LiveStreamsPerClient,
				Total:        requestLimit.LiveStreamsTotal,
			},
		),
	)

	requestRateLimitMiddleware := middlewares.NewRequestRateLimitMiddleware(requestAdmissionApplication)
	engine.Use(
		middlewares.NewRequestBodyLimitMiddleware(requestLimit.BodyLimitBytes).Handle,
		requestRateLimitMiddleware.Handle,
	)

	return requestGuards{
		credentialRequest: requestRateLimitMiddleware.HandleCredentialRequest,
		liveStream:        middlewares.NewLiveStreamLimitMiddleware(requestAdmissionApplication).Handle,
	}
}

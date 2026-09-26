package controller

import (
	"errors"
	"log"
	"net/http"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller/middlewares"
	"github.com/CodeMachine0121/go-trading/internal/controller/models"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/gin-gonic/gin"
)

// The underlying failure is logged, never answered, so storage and driver details stay on the server.
const connectorAuthorizationServerFailureDescription = "伺服器發生錯誤，請稍後再試"

// ConnectorAuthorizationController speaks OAuth (snake_case, {"error","error_description"}) to connectors, and the project's usual JSON to the web front end.
type ConnectorAuthorizationController struct {
	connectorAuthorizationApplication *application.ConnectorAuthorizationApplication
}

func NewConnectorAuthorizationController(
	connectorAuthorizationApplication *application.ConnectorAuthorizationApplication,
) *ConnectorAuthorizationController {
	return &ConnectorAuthorizationController{
		connectorAuthorizationApplication: connectorAuthorizationApplication,
	}
}

// DescribeAuthorizationServer handles GET /.well-known/oauth-authorization-server.
func (connectorAuthorizationController *ConnectorAuthorizationController) DescribeAuthorizationServer(
	ginContext *gin.Context,
) {
	ginContext.JSON(http.StatusOK,
		connectorAuthorizationController.connectorAuthorizationApplication.DescribeAuthorizationServer())
}

// RegisterConnectorClient handles POST /oauth/register.
func (connectorAuthorizationController *ConnectorAuthorizationController) RegisterConnectorClient(
	ginContext *gin.Context,
) {
	var registrationRequest models.ConnectorClientRegistrationRequest

	if bindError := ginContext.ShouldBindJSON(&registrationRequest); bindError != nil {
		connectorAuthorizationController.respondWithProtocolError(
			ginContext, domains.ErrConnectorClientMetadataInvalid)
		return
	}

	connectorClientDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		RegisterConnectorClient(ginContext.Request.Context(), registrationRequest.ToRegistrationDto())
	if err != nil {
		connectorAuthorizationController.respondWithProtocolError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusCreated, connectorClientDto)
}

// StartConnectorAuthorization handles GET /oauth/authorize.
func (connectorAuthorizationController *ConnectorAuthorizationController) StartConnectorAuthorization(
	ginContext *gin.Context,
) {
	redirectDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		StartConnectorAuthorization(ginContext.Request.Context(), dto.ConnectorAuthorizationStartDto{
			ResponseType:        ginContext.Query("response_type"),
			ClientIdentifier:    ginContext.Query("client_id"),
			RedirectUri:         ginContext.Query("redirect_uri"),
			CodeChallenge:       ginContext.Query("code_challenge"),
			CodeChallengeMethod: ginContext.Query("code_challenge_method"),
			State:               ginContext.Query("state"),
			Resource:            ginContext.Query("resource"),
		})
	if err != nil {
		connectorAuthorizationController.respondWithProtocolError(ginContext, err)
		return
	}

	ginContext.Redirect(http.StatusFound, redirectDto.RedirectTo)
}

// GetConnectorAuthorizationRequest handles GET /oauth/authorization-requests/:requestId.
func (connectorAuthorizationController *ConnectorAuthorizationController) GetConnectorAuthorizationRequest(
	ginContext *gin.Context,
) {
	authorizationRequestDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		GetConnectorAuthorizationRequest(ginContext.Request.Context(), ginContext.Param("requestId"))
	if err != nil {
		connectorAuthorizationController.respondWithDecisionError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, authorizationRequestDto)
}

// ApproveConnectorAuthorization handles POST /oauth/authorization-requests/:requestId/approval.
func (connectorAuthorizationController *ConnectorAuthorizationController) ApproveConnectorAuthorization(
	ginContext *gin.Context,
) {
	redirectDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		ApproveConnectorAuthorization(ginContext.Request.Context(), ginContext.Param("requestId"),
			middlewares.CurrentUserID(ginContext))
	if err != nil {
		connectorAuthorizationController.respondWithDecisionError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, redirectDto)
}

// DenyConnectorAuthorization handles POST /oauth/authorization-requests/:requestId/denial; no sign-in, since denying grants nothing.
func (connectorAuthorizationController *ConnectorAuthorizationController) DenyConnectorAuthorization(
	ginContext *gin.Context,
) {
	redirectDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		DenyConnectorAuthorization(ginContext.Request.Context(), ginContext.Param("requestId"))
	if err != nil {
		connectorAuthorizationController.respondWithDecisionError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, redirectDto)
}

// IssueConnectorTokens handles POST /oauth/token; no answer from it may be cached, successful or not.
func (connectorAuthorizationController *ConnectorAuthorizationController) IssueConnectorTokens(
	ginContext *gin.Context,
) {
	ginContext.Header("Cache-Control", "no-store")

	var tokenRequest models.ConnectorTokenRequest

	if bindError := ginContext.ShouldBind(&tokenRequest); bindError != nil || tokenRequest.GrantType == "" {
		connectorAuthorizationController.respondWithProtocolError(ginContext, domains.ErrConnectorTokenRequestInvalid)
		return
	}

	executionContext := ginContext.Request.Context()
	switch tokenRequest.GrantType {
	case "authorization_code":
		tokensDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
			ExchangeAuthorizationCode(executionContext, tokenRequest.ToCodeExchangeDto())
		if err != nil {
			connectorAuthorizationController.respondWithProtocolError(ginContext, err)
			return
		}
		ginContext.JSON(http.StatusOK, tokensDto)
	case "refresh_token":
		tokensDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
			RenewConnectorSession(executionContext, tokenRequest.ToRenewalDto())
		if err != nil {
			connectorAuthorizationController.respondWithProtocolError(ginContext, err)
			return
		}
		ginContext.JSON(http.StatusOK, tokensDto)
	default:
		ginContext.JSON(http.StatusBadRequest, gin.H{
			"error":             "unsupported_grant_type",
			"error_description": "只支援 authorization_code 與 refresh_token",
		})
	}
}

// IntrospectAccessToken handles POST /oauth/introspection.
func (connectorAuthorizationController *ConnectorAuthorizationController) IntrospectAccessToken(
	ginContext *gin.Context,
) {
	var introspectionRequest models.AccessTokenIntrospectionRequest

	if bindError := ginContext.ShouldBind(&introspectionRequest); bindError != nil {
		connectorAuthorizationController.respondWithProtocolError(ginContext, domains.ErrConnectorTokenRequestInvalid)
		return
	}

	introspectionDto, err := connectorAuthorizationController.connectorAuthorizationApplication.
		IntrospectAccessToken(ginContext.Request.Context(), introspectionRequest.Token)
	if err != nil {
		connectorAuthorizationController.respondWithProtocolError(ginContext, err)
		return
	}

	ginContext.JSON(http.StatusOK, introspectionDto)
}

func (connectorAuthorizationController *ConnectorAuthorizationController) respondWithProtocolError(
	ginContext *gin.Context, err error,
) {
	status, code := http.StatusBadRequest, ""
	switch {
	case errors.Is(err, domains.ErrConnectorRedirectUriInvalid):
		code = "invalid_redirect_uri"
	case errors.Is(err, domains.ErrConnectorClientMetadataInvalid):
		code = "invalid_client_metadata"
	case errors.Is(err, domains.ErrConnectorClientNotFound):
		code = "invalid_client"
	case errors.Is(err, domains.ErrConnectorRedirectUriNotRegistered),
		errors.Is(err, domains.ErrConnectorTokenRequestInvalid):
		code = "invalid_request"
	case errors.Is(err, domains.ErrConnectorGrantInvalid),
		errors.Is(err, domains.ErrAuthenticationRequired):
		code = "invalid_grant"
	case errors.Is(err, domains.ErrAccessTokenUnavailable):
		status, code = http.StatusServiceUnavailable, "temporarily_unavailable"
	default:
		log.Printf("connector authorization: %s %s failed: %v",
			ginContext.Request.Method, ginContext.FullPath(), err)
		ginContext.JSON(http.StatusInternalServerError, gin.H{
			"error": "server_error", "error_description": connectorAuthorizationServerFailureDescription,
		})
		return
	}

	ginContext.JSON(status, gin.H{"error": code, "error_description": err.Error()})
}

func (connectorAuthorizationController *ConnectorAuthorizationController) respondWithDecisionError(
	ginContext *gin.Context, err error,
) {
	if errors.Is(err, domains.ErrConnectorAuthorizationRequestNotFound) {
		ginContext.JSON(http.StatusNotFound, gin.H{"message": err.Error()})
		return
	}

	log.Printf("connector authorization: %s %s failed: %v",
		ginContext.Request.Method, ginContext.FullPath(), err)
	ginContext.JSON(http.StatusInternalServerError, gin.H{"message": connectorAuthorizationServerFailureDescription})
}

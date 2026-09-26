package service

import (
	"context"
	"errors"
	"net/url"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ConnectorAuthorizationService lets a connector act for a user without ever seeing the password; the sign-in it opens is an ordinary session chain.
type ConnectorAuthorizationService struct {
	connectorClientRepository               domaininterface.IConnectorClientRepository
	connectorAuthorizationRequestRepository domaininterface.IConnectorAuthorizationRequestRepository
	connectorAuthorizationCodeRepository    domaininterface.IConnectorAuthorizationCodeRepository
	sessionRepository                       domaininterface.ISessionRepository
	userRepository                          domaininterface.IUserRepository
	accessTokenProxy                        domaininterface.IAccessTokenProxy
	refreshTokenProxy                       domaininterface.IRefreshTokenProxy
	clockProxy                              domaininterface.IClockProxy
	sessionLifetimes                        vo.SessionLifetimesVo
	authorizationPolicy                     vo.ConnectorAuthorizationPolicyVo
}

func NewConnectorAuthorizationService(
	connectorClientRepository domaininterface.IConnectorClientRepository,
	connectorAuthorizationRequestRepository domaininterface.IConnectorAuthorizationRequestRepository,
	connectorAuthorizationCodeRepository domaininterface.IConnectorAuthorizationCodeRepository,
	sessionRepository domaininterface.ISessionRepository,
	userRepository domaininterface.IUserRepository,
	accessTokenProxy domaininterface.IAccessTokenProxy,
	refreshTokenProxy domaininterface.IRefreshTokenProxy,
	clockProxy domaininterface.IClockProxy,
	sessionLifetimes vo.SessionLifetimesVo,
	authorizationPolicy vo.ConnectorAuthorizationPolicyVo,
) *ConnectorAuthorizationService {
	return &ConnectorAuthorizationService{
		connectorClientRepository:               connectorClientRepository,
		connectorAuthorizationRequestRepository: connectorAuthorizationRequestRepository,
		connectorAuthorizationCodeRepository:    connectorAuthorizationCodeRepository,
		sessionRepository:                       sessionRepository,
		userRepository:                          userRepository,
		accessTokenProxy:                        accessTokenProxy,
		refreshTokenProxy:                       refreshTokenProxy,
		clockProxy:                              clockProxy,
		sessionLifetimes:                        sessionLifetimes,
		authorizationPolicy:                     authorizationPolicy,
	}
}

func (connectorAuthorizationService *ConnectorAuthorizationService) DescribeAuthorizationServer() dto.ConnectorAuthorizationServerMetadataDto {
	return connectorAuthorizationService.authorizationPolicy.ToServerMetadataDto()
}

func (connectorAuthorizationService *ConnectorAuthorizationService) RegisterConnectorClient(
	executionContext context.Context, registrationDto dto.ConnectorClientRegistrationDto,
) (dto.ConnectorClientDto, error) {
	registration, validationError := domains.NewConnectorClientRegistrationDomain(registrationDto)
	if validationError != nil {
		return dto.ConnectorClientDto{}, validationError
	}

	clientIdentifier, mintError := connectorAuthorizationService.refreshTokenProxy.Mint()
	if mintError != nil {
		return dto.ConnectorClientDto{}, mintError
	}

	savedClient, saveError := connectorAuthorizationService.connectorClientRepository.Save(
		executionContext,
		registration.ToEntity(clientIdentifier.Value, connectorAuthorizationService.clockProxy.Now()))
	if saveError != nil {
		return dto.ConnectorClientDto{}, saveError
	}

	return savedClient.ToDto(), nil
}

// StartConnectorAuthorization refuses outright while the connector or its address is untrusted, and only then may send the browser back with an error.
func (connectorAuthorizationService *ConnectorAuthorizationService) StartConnectorAuthorization(
	executionContext context.Context, startDto dto.ConnectorAuthorizationStartDto,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	connectorClient, findError := connectorAuthorizationService.connectorClientRepository.FindOneByClientIdentifier(
		executionContext, startDto.ClientIdentifier)
	if findError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, findError
	}

	redirectUri, redirectUriError := domains.NewConnectorClientDomain(connectorClient).
		RegisteredRedirectUri(startDto.RedirectUri)
	if redirectUriError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, redirectUriError
	}

	start := domains.NewConnectorAuthorizationStartDomain(startDto, redirectUri)
	if refusal, refused := start.RefusalRedirect(); refused {
		return refusal, nil
	}

	requestIdentifier, mintError := connectorAuthorizationService.refreshTokenProxy.Mint()
	if mintError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, mintError
	}

	if _, saveError := connectorAuthorizationService.connectorAuthorizationRequestRepository.Save(
		executionContext,
		start.ToEntity(requestIdentifier.Value, connectorAuthorizationService.clockProxy.Now(),
			connectorAuthorizationService.authorizationPolicy.RequestLifetime),
	); saveError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, saveError
	}

	return dto.ConnectorAuthorizationRedirectDto{
		RedirectTo: connectorAuthorizationService.authorizationPolicy.FrontendBaseUrl +
			"/connector-authorization?" + url.Values{"request": {requestIdentifier.Value}}.Encode(),
	}, nil
}

func (connectorAuthorizationService *ConnectorAuthorizationService) GetConnectorAuthorizationRequest(
	executionContext context.Context, requestIdentifier string,
) (dto.ConnectorAuthorizationRequestDto, error) {
	authorizationRequest, openError := connectorAuthorizationService.openAuthorizationRequest(
		executionContext, requestIdentifier)
	if openError != nil {
		return dto.ConnectorAuthorizationRequestDto{}, openError
	}

	connectorClient, findError := connectorAuthorizationService.connectorClientRepository.FindOneByClientIdentifier(
		executionContext, authorizationRequest.ConnectorClientIdentifier())
	if errors.Is(findError, domains.ErrConnectorClientNotFound) {
		return dto.ConnectorAuthorizationRequestDto{}, domains.ErrConnectorAuthorizationRequestNotFound
	}
	if findError != nil {
		return dto.ConnectorAuthorizationRequestDto{}, findError
	}

	return authorizationRequest.ToDto(connectorClient.ClientName), nil
}

// ApproveConnectorAuthorization issues a single-use code bound to the user; the code and the decision are written together so neither survives alone.
func (connectorAuthorizationService *ConnectorAuthorizationService) ApproveConnectorAuthorization(
	executionContext context.Context, requestIdentifier string, userID uint,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	authorizationRequest, openError := connectorAuthorizationService.openAuthorizationRequest(
		executionContext, requestIdentifier)
	if openError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, openError
	}

	authorizationCode, mintError := connectorAuthorizationService.refreshTokenProxy.Mint()
	if mintError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, mintError
	}

	approvalRedirect, redirectError := authorizationRequest.ApprovalRedirect(authorizationCode.Value)
	if redirectError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, redirectError
	}

	now := connectorAuthorizationService.clockProxy.Now()
	if approveError := connectorAuthorizationService.connectorAuthorizationRequestRepository.Approve(
		executionContext,
		authorizationRequest.ID(),
		now,
		authorizationRequest.ToAuthorizationCode(userID, authorizationCode.Digest, now,
			connectorAuthorizationService.authorizationPolicy.CodeLifetime),
	); approveError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, approveError
	}

	return approvalRedirect, nil
}

func (connectorAuthorizationService *ConnectorAuthorizationService) DenyConnectorAuthorization(
	executionContext context.Context, requestIdentifier string,
) (dto.ConnectorAuthorizationRedirectDto, error) {
	authorizationRequest, openError := connectorAuthorizationService.openAuthorizationRequest(
		executionContext, requestIdentifier)
	if openError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, openError
	}

	denialRedirect, redirectError := authorizationRequest.DenialRedirect()
	if redirectError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, redirectError
	}

	if denyError := connectorAuthorizationService.connectorAuthorizationRequestRepository.Deny(
		executionContext, authorizationRequest.ID(), connectorAuthorizationService.clockProxy.Now(),
	); denyError != nil {
		return dto.ConnectorAuthorizationRedirectDto{}, denyError
	}

	return denialRedirect, nil
}

// openAuthorizationRequest answers unknown, expired and decided with the same error so nothing can be probed.
func (connectorAuthorizationService *ConnectorAuthorizationService) openAuthorizationRequest(
	executionContext context.Context, requestIdentifier string,
) (domains.ConnectorAuthorizationRequestDomain, error) {
	storedRequest, findError := connectorAuthorizationService.connectorAuthorizationRequestRepository.
		FindOneByRequestIdentifier(executionContext, requestIdentifier)
	if findError != nil {
		return domains.ConnectorAuthorizationRequestDomain{}, findError
	}

	authorizationRequest := domains.NewConnectorAuthorizationRequestDomain(storedRequest)
	if !authorizationRequest.Open(connectorAuthorizationService.clockProxy.Now()) {
		return domains.ConnectorAuthorizationRequestDomain{}, domains.ErrConnectorAuthorizationRequestNotFound
	}

	return authorizationRequest, nil
}

// ExchangeAuthorizationCode opens a new session chain for the code's user; a replayed code means it leaked, so the chain it produced is torn down.
func (connectorAuthorizationService *ConnectorAuthorizationService) ExchangeAuthorizationCode(
	executionContext context.Context, exchangeDto dto.ConnectorAuthorizationCodeExchangeDto,
) (dto.ConnectorTokensDto, error) {
	exchange, validationError := domains.NewConnectorAuthorizationCodeExchangeDomain(exchangeDto)
	if validationError != nil {
		return dto.ConnectorTokensDto{}, validationError
	}

	if _, clientError := connectorAuthorizationService.connectorClientRepository.FindOneByClientIdentifier(
		executionContext, exchange.ClientIdentifier()); clientError != nil {
		return dto.ConnectorTokensDto{}, clientError
	}

	codeDigest := connectorAuthorizationService.refreshTokenProxy.DigestOf(exchange.Code())
	storedCode, findError := connectorAuthorizationService.connectorAuthorizationCodeRepository.FindOneByDigest(
		executionContext, codeDigest)
	if errors.Is(findError, domains.ErrConnectorAuthorizationCodeNotFound) {
		return dto.ConnectorTokensDto{}, domains.ErrConnectorGrantInvalid
	}
	if findError != nil {
		return dto.ConnectorTokensDto{}, findError
	}

	authorizationCode := domains.NewConnectorAuthorizationCodeDomain(storedCode)
	if authorizationCode.Redeemed() {
		return dto.ConnectorTokensDto{}, connectorAuthorizationService.revokedReplay(
			executionContext, authorizationCode.SessionChainID())
	}

	now := connectorAuthorizationService.clockProxy.Now()
	if authorizationCode.Expired(now) || !authorizationCode.Accepts(exchange) {
		return dto.ConnectorTokensDto{}, domains.ErrConnectorGrantInvalid
	}

	if _, userError := connectorAuthorizationService.userRepository.FindOne(
		executionContext, authorizationCode.UserID()); userError != nil {
		if errors.Is(userError, domains.ErrUserNotFound) {
			return dto.ConnectorTokensDto{}, domains.ErrConnectorGrantInvalid
		}

		return dto.ConnectorTokensDto{}, userError
	}

	refreshToken, mintError := connectorAuthorizationService.refreshTokenProxy.Mint()
	if mintError != nil {
		return dto.ConnectorTokensDto{}, mintError
	}

	accessToken, issueError := connectorAuthorizationService.accessTokenProxy.Issue(vo.AccessTokenClaimsVo{
		UserID:    authorizationCode.UserID(),
		Audience:  authorizationCode.Audience(),
		ExpiresAt: now.Add(connectorAuthorizationService.sessionLifetimes.AccessToken),
	})
	if issueError != nil {
		return dto.ConnectorTokensDto{}, issueError
	}

	savedSession, redeemError := connectorAuthorizationService.connectorAuthorizationCodeRepository.Redeem(
		executionContext,
		authorizationCode.ID(),
		authorizationCode.ToSession(refreshToken.Digest, now,
			connectorAuthorizationService.sessionLifetimes.RefreshToken),
	)
	if errors.Is(redeemError, domains.ErrConnectorAuthorizationCodeAlreadyRedeemed) {
		// Lost a race with a concurrent exchange; the winner's chain is only known after it committed.
		redeemedCode, rereadError := connectorAuthorizationService.connectorAuthorizationCodeRepository.
			FindOneByDigest(executionContext, codeDigest)
		if rereadError != nil {
			return dto.ConnectorTokensDto{}, rereadError
		}

		return dto.ConnectorTokensDto{}, connectorAuthorizationService.revokedReplay(
			executionContext, domains.NewConnectorAuthorizationCodeDomain(redeemedCode).SessionChainID())
	}
	if redeemError != nil {
		return dto.ConnectorTokensDto{}, redeemError
	}

	return vo.SessionTokensVo{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: savedSession.ExpiresAt,
	}.ToConnectorTokensDto(now), nil
}

// revokedReplay tears down the chain a replayed code produced; failing to revoke fails the request.
func (connectorAuthorizationService *ConnectorAuthorizationService) revokedReplay(
	executionContext context.Context, sessionChainID string,
) error {
	if sessionChainID != "" {
		if revokeError := connectorAuthorizationService.sessionRepository.RevokeChain(
			executionContext, sessionChainID); revokeError != nil {
			return revokeError
		}
	}

	return domains.ErrConnectorGrantInvalid
}

// IntrospectAccessToken says nothing about why a token is inactive.
func (connectorAuthorizationService *ConnectorAuthorizationService) IntrospectAccessToken(
	executionContext context.Context, accessToken string,
) (dto.AccessTokenIntrospectionDto, error) {
	if accessToken == "" {
		return dto.AccessTokenIntrospectionDto{Active: false}, nil
	}

	claims, claimsError := connectorAuthorizationService.accessTokenProxy.ClaimsOf(accessToken)
	if claimsError != nil {
		return dto.AccessTokenIntrospectionDto{Active: false}, nil
	}

	_, findError := connectorAuthorizationService.userRepository.FindOne(executionContext, claims.UserID)
	if errors.Is(findError, domains.ErrUserNotFound) {
		return dto.AccessTokenIntrospectionDto{Active: false}, nil
	}
	if findError != nil {
		return dto.AccessTokenIntrospectionDto{}, findError
	}

	return claims.ToIntrospectionDto(), nil
}

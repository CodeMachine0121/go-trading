package service

import (
	"context"
	"errors"
	"log"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// UserService owns registration, sign-in and identification.
type UserService struct {
	userRepository     domaininterface.IUserRepository
	sessionRepository  domaininterface.ISessionRepository
	passwordProofProxy domaininterface.IPasswordProofProxy
	accessTokenProxy   domaininterface.IAccessTokenProxy
	refreshTokenProxy  domaininterface.IRefreshTokenProxy
	clockProxy         domaininterface.IClockProxy
	sessionLifetimes   vo.SessionLifetimesVo
	activationPolicy   vo.AccountActivationPolicyVo
	lockoutPolicy      vo.SignInLockoutPolicyVo
}

func NewUserService(
	userRepository domaininterface.IUserRepository,
	sessionRepository domaininterface.ISessionRepository,
	passwordProofProxy domaininterface.IPasswordProofProxy,
	accessTokenProxy domaininterface.IAccessTokenProxy,
	refreshTokenProxy domaininterface.IRefreshTokenProxy,
	clockProxy domaininterface.IClockProxy,
	sessionLifetimes vo.SessionLifetimesVo,
	activationPolicy vo.AccountActivationPolicyVo,
	lockoutPolicy vo.SignInLockoutPolicyVo,
) *UserService {
	return &UserService{
		userRepository:     userRepository,
		sessionRepository:  sessionRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
		refreshTokenProxy:  refreshTokenProxy,
		clockProxy:         clockProxy,
		sessionLifetimes:   sessionLifetimes,
		activationPolicy:   activationPolicy,
		lockoutPolicy:      lockoutPolicy,
	}
}

// RegisterUser validates before hashing or writing, and returns the user awaiting activation along with what to do about it.
func (userService *UserService) RegisterUser(
	executionContext context.Context, registrationDto dto.UserRegistrationDto,
) (dto.UserDto, error) {
	registration, validationError := domains.NewUserRegistrationDomain(registrationDto)
	if validationError != nil {
		return dto.UserDto{}, validationError
	}

	passwordProof, proveError := userService.passwordProofProxy.Prove(registration.Password())
	if proveError != nil {
		return dto.UserDto{}, proveError
	}

	// Uniqueness is enforced by the write, not a prior lookup, so concurrent registrations cannot both pass.
	savedUser, saveError := userService.userRepository.Save(
		executionContext, registration.ToEntity(passwordProof))
	if saveError != nil {
		return dto.UserDto{}, saveError
	}

	return domains.NewAccountActivationDomain(savedUser, userService.activationPolicy).ToUserDto(), nil
}

// signInFailureCountingAttempts bounds optimistic-concurrency retries when counting a failed sign-in; it matches the lockout threshold so the retries cannot all be lost.
const signInFailureCountingAttempts = 3

// SignIn verifies credentials and stores a new session, returning its access and renewal tokens.
func (userService *UserService) SignIn(
	executionContext context.Context, signInDto dto.SignInDto,
) (dto.SessionTokensDto, error) {
	signIn, credentialsError := domains.NewSignInDomain(signInDto)
	if credentialsError != nil {
		return dto.SessionTokensDto{}, credentialsError
	}

	// A missing account is not returned early so the password check still runs (no timing leak on unregistered addresses), whereas a storage error is surfaced.
	user, findError := userService.userRepository.FindOneByEmail(executionContext, signIn.Email())
	if findError != nil && !errors.Is(findError, domains.ErrUserNotFound) {
		return dto.SessionTokensDto{}, findError
	}

	now := userService.clockProxy.Now()
	lockout := domains.NewSignInLockoutDomain(user, userService.lockoutPolicy, now)

	// Checked before the bcrypt comparison so a locked account costs nothing and retries cannot extend the lock.
	if refusal := lockout.Refusal(); refusal != nil {
		return dto.SessionTokensDto{}, refusal
	}

	if !userService.passwordProofProxy.Matches(signIn.Password(), user.PasswordProof) {
		if recordError := userService.countFailedSignIn(executionContext, user); recordError != nil {
			return dto.SessionTokensDto{}, recordError
		}

		return dto.SessionTokensDto{}, domains.ErrCredentialsRejected
	}

	// A failure to record the outcome fails the sign-in; swallowing it would silently disable the lockout.
	if recordError := userService.recordSignInOutcome(
		executionContext, user.ID, user.FailedSignInCount,
		lockout.AfterSuccess()); recordError != nil {
		return dto.SessionTokensDto{}, recordError
	}

	refreshToken, accessToken, materialError := userService.newSessionMaterial(user.ID, "", now)
	if materialError != nil {
		return dto.SessionTokensDto{}, materialError
	}

	// A new chain is identified by the renewal-token digest, which is already unique and random.
	savedSession, saveError := userService.sessionRepository.Save(executionContext, entities.Session{
		UserID:             user.ID,
		ChainID:            refreshToken.Digest,
		RefreshTokenDigest: refreshToken.Digest,
		ExpiresAt:          now.Add(userService.sessionLifetimes.RefreshToken).UTC(),
	})
	if saveError != nil {
		return dto.SessionTokensDto{}, saveError
	}

	return vo.SessionTokensVo{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: savedSession.ExpiresAt,
	}.ToDto(), nil
}

// recordSignInOutcome is a no-op for an unregistered address (user ID zero), so it leaves no trace and reads like any other wrong pair.
func (userService *UserService) recordSignInOutcome(
	executionContext context.Context,
	userID uint,
	observedFailedSignInCount int,
	state vo.SignInLockoutStateVo,
) error {
	if userID == 0 {
		return nil
	}

	return userService.userRepository.SaveSignInLockoutState(
		executionContext, userID, observedFailedSignInCount, state)
}

// countFailedSignIn retries on write conflicts so parallel guesses cannot collapse into one increment; running out of retries is an error, not forgiven.
func (userService *UserService) countFailedSignIn(
	executionContext context.Context, user entities.User,
) error {
	for range signInFailureCountingAttempts {
		lockout := domains.NewSignInLockoutDomain(user, userService.lockoutPolicy,
			userService.clockProxy.Now())

		// Already locked by a concurrent attempt; counting again would push the lock end further out.
		if lockout.Refusal() != nil {
			return nil
		}

		nextStanding := lockout.AfterFailure()

		recordError := userService.recordSignInOutcome(
			executionContext, user.ID, user.FailedSignInCount, nextStanding)
		if !errors.Is(recordError, domains.ErrSignInLockoutStateStale) {
			if recordError == nil && nextStanding.LockedUntil != nil {
				// Log only when the account locks, not every failure.
				log.Printf("sign in lockout: account %d shut until %s after %d consecutive failures",
					user.ID, nextStanding.LockedUntil.Format(time.RFC3339),
					nextStanding.FailedSignInCount)
			}

			return recordError
		}

		freshUser, findError := userService.userRepository.FindOne(executionContext, user.ID)
		if findError != nil {
			return findError
		}
		user = freshUser
	}

	// Retries exhausted: if the reloaded row is now locked, this attempt is accounted for.
	if domains.NewSignInLockoutDomain(
		user, userService.lockoutPolicy, userService.clockProxy.Now()).Refusal() != nil {
		return nil
	}

	return domains.ErrSignInLockoutStateStale
}

// RenewSession rotates a web renewal token; reuse of an already-rotated token signals theft, so the whole chain is revoked.
func (userService *UserService) RenewSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) (dto.SessionTokensDto, error) {
	sessionTokens, _, renewError := userService.renewedSessionTokens(executionContext, dto.SessionRenewalDto{
		RefreshToken: renewalDto.RefreshToken,
	})
	if renewError != nil {
		return dto.SessionTokensDto{}, renewError
	}

	return sessionTokens.ToDto(), nil
}

// RenewConnectorSession rotates a connector's renewal token under the same rules as the web, keeping the session's audience on the new access token.
func (userService *UserService) RenewConnectorSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) (dto.ConnectorTokensDto, error) {
	if renewalDto.RefreshToken == "" || renewalDto.ConnectorClientIdentifier == "" {
		return dto.ConnectorTokensDto{}, domains.ErrConnectorTokenRequestInvalid
	}

	sessionTokens, now, renewError := userService.renewedSessionTokens(executionContext, renewalDto)
	if renewError != nil {
		return dto.ConnectorTokensDto{}, renewError
	}

	return sessionTokens.ToConnectorTokensDto(now), nil
}

func (userService *UserService) renewedSessionTokens(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) (vo.SessionTokensVo, time.Time, error) {
	storedSession, sessionExists, findError := userService.sessionHolding(
		executionContext, renewalDto.RefreshToken)
	if findError != nil {
		return vo.SessionTokensVo{}, time.Time{}, findError
	}
	if !sessionExists {
		return vo.SessionTokensVo{}, time.Time{}, domains.ErrAuthenticationRequired
	}

	session := domains.NewSessionDomain(storedSession)
	now := userService.clockProxy.Now()

	if session.Revoked() {
		return vo.SessionTokensVo{}, time.Time{}, userService.tornDownChain(executionContext, session.ChainID())
	}

	// A token presented through the other channel is refused without teardown: it is misuse, not proof of theft.
	if !session.HeldBy(renewalDto.ConnectorClientIdentifier) {
		return vo.SessionTokensVo{}, time.Time{}, domains.ErrAuthenticationRequired
	}

	if session.Expired(now) {
		// Expiry is not theft, so the chain is left intact.
		return vo.SessionTokensVo{}, time.Time{}, domains.ErrAuthenticationRequired
	}

	if _, userError := userService.userRepository.FindOne(
		executionContext, session.UserID()); userError != nil {
		if errors.Is(userError, domains.ErrUserNotFound) {
			return vo.SessionTokensVo{}, time.Time{}, domains.ErrAuthenticationRequired
		}

		return vo.SessionTokensVo{}, time.Time{}, userError
	}

	refreshToken, accessToken, materialError := userService.newSessionMaterial(
		session.UserID(), session.Audience(), now)
	if materialError != nil {
		return vo.SessionTokensVo{}, time.Time{}, materialError
	}

	rotatedSession, rotateError := userService.sessionRepository.Rotate(
		executionContext,
		session.ID(),
		session.Renewed(refreshToken.Digest, now, userService.sessionLifetimes.RefreshToken),
	)
	// A rotation refused by the store means a concurrent renewal or sign-out used this token, so treat it as reuse and revoke the chain.
	if errors.Is(rotateError, domains.ErrSessionAlreadyRotated) {
		return vo.SessionTokensVo{}, time.Time{}, userService.tornDownChain(executionContext, session.ChainID())
	}
	if rotateError != nil {
		return vo.SessionTokensVo{}, time.Time{}, rotateError
	}

	return vo.SessionTokensVo{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: rotatedSession.ExpiresAt,
	}, now, nil
}

// RevokeSession ends the sign-in a renewal token belongs to; an unknown token is success, and the stateless access token stays valid until it expires.
func (userService *UserService) RevokeSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) error {
	storedSession, sessionExists, findError := userService.sessionHolding(
		executionContext, renewalDto.RefreshToken)
	if findError != nil {
		return findError
	}
	if !sessionExists {
		return nil
	}

	// Revoke the whole chain, i.e. the device's sign-in, not just this session.
	return userService.sessionRepository.RevokeChain(executionContext, storedSession.ChainID)
}

// tornDownChain revokes a chain after detected token reuse; failing to revoke fails the request.
func (userService *UserService) tornDownChain(
	executionContext context.Context, chainID string,
) error {
	if revokeError := userService.sessionRepository.RevokeChain(
		executionContext, chainID); revokeError != nil {
		return revokeError
	}

	return domains.ErrAuthenticationRequired
}

// sessionHolding looks up the session for a renewal token, reporting "not found" separately because callers react to it differently; an empty token skips storage.
func (userService *UserService) sessionHolding(
	executionContext context.Context, refreshToken string,
) (entities.Session, bool, error) {
	if refreshToken == "" {
		return entities.Session{}, false, nil
	}

	storedSession, findError := userService.sessionRepository.FindOneByDigest(
		executionContext, userService.refreshTokenProxy.DigestOf(refreshToken))
	if errors.Is(findError, domains.ErrSessionNotFound) {
		return entities.Session{}, false, nil
	}
	if findError != nil {
		return entities.Session{}, false, findError
	}

	return storedSession, true, nil
}

// newSessionMaterial mints and signs tokens before any write, so a failure leaves no session behind and does not end the caller's existing one.
func (userService *UserService) newSessionMaterial(
	userID uint, audience string, now time.Time,
) (vo.RefreshTokenVo, vo.AccessTokenVo, error) {
	refreshToken, mintError := userService.refreshTokenProxy.Mint()
	if mintError != nil {
		return vo.RefreshTokenVo{}, vo.AccessTokenVo{}, mintError
	}

	accessToken, issueError := userService.accessTokenProxy.Issue(vo.AccessTokenClaimsVo{
		UserID:    userID,
		Audience:  audience,
		ExpiresAt: now.Add(userService.sessionLifetimes.AccessToken),
	})
	if issueError != nil {
		return vo.RefreshTokenVo{}, vo.AccessTokenVo{}, issueError
	}

	return refreshToken, accessToken, nil
}

// IdentifyUser resolves a token's user even if not yet activated, so a waiting user can check their status.
func (userService *UserService) IdentifyUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	user, identifyError := userService.identifiedUser(executionContext, accessToken)
	if identifyError != nil {
		return dto.UserDto{}, identifyError
	}

	return domains.NewAccountActivationDomain(user, userService.activationPolicy).ToUserDto(), nil
}

// IdentifyActivatedUser is the single activation check every endpoint uses; identification happens first so a broken token is not reported as "not activated".
func (userService *UserService) IdentifyActivatedUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	return userService.activatedUser(executionContext, accessToken)
}

// IdentifyActivatedWebUser refuses connector tokens, so a connector cannot act where only the user in the browser may.
func (userService *UserService) IdentifyActivatedWebUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	claims, claimsError := userService.accessTokenProxy.ClaimsOf(accessToken)
	if claimsError != nil {
		return dto.UserDto{}, claimsError
	}
	if claims.Audience != "" {
		return dto.UserDto{}, domains.ErrAuthenticationRequired
	}

	return userService.activatedUser(executionContext, accessToken)
}

func (userService *UserService) activatedUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	user, identifyError := userService.identifiedUser(executionContext, accessToken)
	if identifyError != nil {
		return dto.UserDto{}, identifyError
	}

	activation := domains.NewAccountActivationDomain(user, userService.activationPolicy)
	if activation.Pending() {
		return dto.UserDto{}, activation.NotActivatedError()
	}

	return activation.ToUserDto(), nil
}

// identifiedUser reads the user from the store rather than the token, so deletion and activation take effect immediately.
func (userService *UserService) identifiedUser(
	executionContext context.Context, accessToken string,
) (entities.User, error) {
	if accessToken == "" {
		return entities.User{}, domains.ErrAuthenticationRequired
	}

	userID, identifyError := userService.accessTokenProxy.UserIdentifiedBy(accessToken)
	if identifyError != nil {
		return entities.User{}, identifyError
	}

	user, findError := userService.userRepository.FindOne(executionContext, userID)
	if errors.Is(findError, domains.ErrUserNotFound) {
		return entities.User{}, domains.ErrAuthenticationRequired
	}
	if findError != nil {
		return entities.User{}, findError
	}

	return user, nil
}

// ChangePassword runs cheap validation before the slow bcrypt check and revokes every open session; the user ID comes from the token, never the request.
func (userService *UserService) ChangePassword(
	executionContext context.Context, userID uint, passwordChangeDto dto.PasswordChangeDto,
) error {
	passwordChange, validationError := domains.NewPasswordChangeDomain(passwordChangeDto)
	if validationError != nil {
		return validationError
	}

	user, findError := userService.userRepository.FindOne(executionContext, userID)
	if errors.Is(findError, domains.ErrUserNotFound) {
		return domains.ErrAuthenticationRequired
	}
	if findError != nil {
		return findError
	}

	// No timing decoy needed here: the user is already identified, so there is nothing to enumerate.
	if !userService.passwordProofProxy.Matches(
		passwordChange.CurrentPassword(), user.PasswordProof) {
		return domains.ErrCurrentPasswordRejected
	}

	newPasswordProof, proveError := userService.passwordProofProxy.Prove(
		passwordChange.NewPassword())
	if proveError != nil {
		return proveError
	}

	return userService.userRepository.ChangePasswordProof(
		executionContext, userID, newPasswordProof)
}

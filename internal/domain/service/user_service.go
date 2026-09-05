package service

import (
	"context"
	"errors"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// UserService is the application layer's only entry point for the people this system
// recognises. Its three public use-case methods never call one another.
//
// Registering, signing in and being recognised live in one service rather than in
// two, because they change for one reason: what it takes to count as somebody this
// system knows. Split apart, both halves would hold the same store and the same
// notion of an account, and no change to either would ever touch only one of them —
// which is a single module written down twice.
type UserService struct {
	userRepository     domaininterface.IUserRepository
	sessionRepository  domaininterface.ISessionRepository
	passwordProofProxy domaininterface.IPasswordProofProxy
	accessTokenProxy   domaininterface.IAccessTokenProxy
	refreshTokenProxy  domaininterface.IRefreshTokenProxy
	clockProxy         domaininterface.IClockProxy
	sessionLifetimes   vo.SessionLifetimesVo
}

func NewUserService(
	userRepository domaininterface.IUserRepository,
	sessionRepository domaininterface.ISessionRepository,
	passwordProofProxy domaininterface.IPasswordProofProxy,
	accessTokenProxy domaininterface.IAccessTokenProxy,
	refreshTokenProxy domaininterface.IRefreshTokenProxy,
	clockProxy domaininterface.IClockProxy,
	sessionLifetimes vo.SessionLifetimesVo,
) *UserService {
	return &UserService{
		userRepository:     userRepository,
		sessionRepository:  sessionRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
		refreshTokenProxy:  refreshTokenProxy,
		clockProxy:         clockProxy,
		sessionLifetimes:   sessionLifetimes,
	}
}

// RegisterUser creates a user and hands them back as stored. A registration that
// breaks a rule is refused before the password is turned into anything and before
// anything is written.
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

	// Whether the address is free is left to the write, not asked beforehand. Asking
	// first would let two registrations arriving at once both find it free.
	savedUser, saveError := userService.userRepository.Save(
		executionContext, registration.ToEntity(passwordProof))
	if saveError != nil {
		return dto.UserDto{}, saveError
	}

	return savedUser.ToDto(), nil
}

// SignIn checks a pair and opens a session, handing back the two proofs that session
// is made of.
//
// It writes now, where the previous version wrote nothing. That is the whole point of
// this slice: something has to be stored for a sign-in to be endable, and the thing
// stored is deliberately the half nobody carries on ordinary requests.
func (userService *UserService) SignIn(
	executionContext context.Context, signInDto dto.SignInDto,
) (dto.SessionTokensDto, error) {
	signIn, credentialsError := domains.NewSignInDomain(signInDto)
	if credentialsError != nil {
		return dto.SessionTokensDto{}, credentialsError
	}

	// Nobody holding this address is not a failure to look, so it is deliberately
	// not returned here. The check below runs either way and refuses either way —
	// with no account there is no proof, and a password checked against no proof is
	// as slow to refuse as one checked against the wrong proof. Turning back early
	// is exactly how "that address is not registered" gets answered in a timing
	// difference nobody wrote down.
	//
	// Storage being broken, on the other hand, is not a wrong password at all.
	// Dressing it up as one would have somebody retyping a password that was right.
	user, findError := userService.userRepository.FindOneByEmail(executionContext, signIn.Email())
	if findError != nil && !errors.Is(findError, domains.ErrUserNotFound) {
		return dto.SessionTokensDto{}, findError
	}

	if !userService.passwordProofProxy.Matches(signIn.Password(), user.PasswordProof) {
		return dto.SessionTokensDto{}, domains.ErrCredentialsRejected
	}

	now := userService.clockProxy.Now()

	refreshToken, accessToken, materialError := userService.newSessionMaterial(user.ID, now)
	if materialError != nil {
		return dto.SessionTokensDto{}, materialError
	}

	// A brand-new sign-in starts a chain, and the chain is known by the digest of the
	// proof that started it. That value is already unique (the column says so) and
	// already random, so minting a second random value here would be two sources of
	// randomness for one fact — and two things that have to agree.
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

// RenewSession trades a renewal proof for a fresh pair, and ends the proof it was
// given in the same breath.
//
// A renewal proof works exactly once. So a proof that has already been used turning
// up again is not a mistake somebody made — it is two copies of it existing, which
// means one of them was taken. There is no way to tell which of the two holders is
// the real one, so the only safe answer is to end the whole chain and make the real
// one sign in again. Refusing just this proof would leave the thief's copy working.
func (userService *UserService) RenewSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) (dto.SessionTokensDto, error) {
	if renewalDto.RefreshToken == "" {
		return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
	}

	storedSession, findError := userService.sessionRepository.FindOneByDigest(
		executionContext, userService.refreshTokenProxy.DigestOf(renewalDto.RefreshToken))
	if errors.Is(findError, domains.ErrSessionNotFound) {
		return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
	}
	if findError != nil {
		return dto.SessionTokensDto{}, findError
	}

	session := domains.NewSessionDomain(storedSession)
	now := userService.clockProxy.Now()

	if session.Revoked() {
		// Tearing the chain down is the answer here, so failing to tear it down is a
		// failure of this request. Reporting "sign in again" while the thief's proof
		// quietly still works would be the worst of both.
		if revokeError := userService.sessionRepository.RevokeChain(
			executionContext, session.ChainID()); revokeError != nil {
			return dto.SessionTokensDto{}, revokeError
		}

		return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
	}

	if session.Expired(now) {
		// Expiry is not theft. The chain stays as it is: there is nothing to
		// tear down, and tearing it down would sign out a second device for no reason.
		return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
	}

	if _, userError := userService.userRepository.FindOne(
		executionContext, session.UserID()); userError != nil {
		if errors.Is(userError, domains.ErrUserNotFound) {
			return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
		}

		return dto.SessionTokensDto{}, userError
	}

	refreshToken, accessToken, materialError := userService.newSessionMaterial(session.UserID(), now)
	if materialError != nil {
		return dto.SessionTokensDto{}, materialError
	}

	rotatedSession, rotateError := userService.sessionRepository.Rotate(
		executionContext,
		session.ID(),
		session.Renewed(refreshToken.Digest, now, userService.sessionLifetimes.RefreshToken),
	)
	if rotateError != nil {
		return dto.SessionTokensDto{}, rotateError
	}

	return vo.SessionTokensVo{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: rotatedSession.ExpiresAt,
	}.ToDto(), nil
}

// RevokeSession ends the sign-in a renewal proof belongs to.
//
// Being handed a proof that matches nothing is success, not failure. What was asked
// for is that this sign-in stop working, and a sign-in that was never there already
// does not work. Reporting an error would have the caller retrying to reach a state
// it is already in.
//
// The access token issued for that session is untouched, because it is not stored
// and cannot be. It keeps working until it expires — which is exactly what its
// lifetime is for, and why it is measured in minutes.
func (userService *UserService) RevokeSession(
	executionContext context.Context, renewalDto dto.SessionRenewalDto,
) error {
	if renewalDto.RefreshToken == "" {
		return nil
	}

	storedSession, findError := userService.sessionRepository.FindOneByDigest(
		executionContext, userService.refreshTokenProxy.DigestOf(renewalDto.RefreshToken))
	if errors.Is(findError, domains.ErrSessionNotFound) {
		return nil
	}
	if findError != nil {
		return findError
	}

	// The whole chain goes, not just this session. A chain is one device's one
	// sign-in, and signing out means that device, not that proof.
	return userService.sessionRepository.RevokeChain(executionContext, storedSession.ChainID)
}

// newSessionMaterial produces the two things opening a session needs, before
// anything at all is written.
//
// The order matters and is the reason this is one helper rather than two calls at
// the call sites. Minting and signing can both fail, and both failing before the
// write means a failed sign-in leaves no session behind, while a failed renewal
// leaves the caller's existing proof still working. Signing after the write would
// end somebody's session and then hand them nothing to replace it with.
//
// It is private and shared by exactly the two public methods that open a session.
func (userService *UserService) newSessionMaterial(
	userID uint, now time.Time,
) (vo.RefreshTokenVo, vo.AccessTokenVo, error) {
	refreshToken, mintError := userService.refreshTokenProxy.Mint()
	if mintError != nil {
		return vo.RefreshTokenVo{}, vo.AccessTokenVo{}, mintError
	}

	accessToken, issueError := userService.accessTokenProxy.Issue(
		userID, now.Add(userService.sessionLifetimes.AccessToken))
	if issueError != nil {
		return vo.RefreshTokenVo{}, vo.AccessTokenVo{}, issueError
	}

	return refreshToken, accessToken, nil
}

// IdentifyUser says who a proof of identity belongs to.
//
// The user is read back rather than taken from the proof, and that is what makes a
// token stop working when the account behind it is gone. A proof carries who it was
// issued to, not whether they are still here — those are different questions, and
// only the store can answer the second.
func (userService *UserService) IdentifyUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	if accessToken == "" {
		return dto.UserDto{}, domains.ErrAuthenticationRequired
	}

	userID, identifyError := userService.accessTokenProxy.UserIdentifiedBy(accessToken)
	if identifyError != nil {
		return dto.UserDto{}, identifyError
	}

	user, findError := userService.userRepository.FindOne(executionContext, userID)
	if errors.Is(findError, domains.ErrUserNotFound) {
		return dto.UserDto{}, domains.ErrAuthenticationRequired
	}
	if findError != nil {
		return dto.UserDto{}, findError
	}

	return user.ToDto(), nil
}

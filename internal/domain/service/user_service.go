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

// RegisterUser creates a user and hands them back as stored, which is waiting to be
// let in. A registration that breaks a rule is refused before the password is turned
// into anything and before anything is written.
//
// The answer carries what to do about the waiting, because this is the one moment the
// person is certain to be looking. Saying it only here would not be enough — they
// will have closed the page long before their first refusal — which is why the same
// instruction comes back with every refusal too.
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

	return domains.NewAccountActivationDomain(savedUser, userService.activationPolicy).ToUserDto(), nil
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

	now := userService.clockProxy.Now()
	lockout := domains.NewSignInLockoutDomain(user, userService.lockoutPolicy, now)

	// Before the comparison, not after. A shut account that still paid for a bcrypt
	// comparison on every attempt would be paying exactly the cost this lock exists
	// to stop — and because the flow turns back here, there is no path from a locked
	// account to the counting below. "Trying again does not extend the lock" is
	// therefore not a rule anybody has to keep; it is a road that is not there.
	//
	// Nobody holding this address reaches this with a zero-valued user, whose lock
	// is absent, so the decoy comparison below still happens and still takes its
	// time. What does not happen is any of the recording: a user identifier of zero
	// matches no row, and an address that is not an account leaves nothing behind.
	if refusal := lockout.Refusal(); refusal != nil {
		return dto.SessionTokensDto{}, refusal
	}

	if !userService.passwordProofProxy.Matches(signIn.Password(), user.PasswordProof) {
		if recordError := userService.recordSignInOutcome(
			executionContext, user.ID, lockout.AfterFailure()); recordError != nil {
			return dto.SessionTokensDto{}, recordError
		}

		return dto.SessionTokensDto{}, domains.ErrCredentialsRejected
	}

	// A failure to write this is a failure to sign in, deliberately. Swallowed, the
	// lock would quietly not exist for as long as the store was unwell, and the one
	// thing nobody would learn is that it had stopped protecting anything.
	if recordError := userService.recordSignInOutcome(
		executionContext, user.ID, lockout.AfterSuccess()); recordError != nil {
		return dto.SessionTokensDto{}, recordError
	}

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

// recordSignInOutcome stores what an attempt left behind, and says nothing at all
// when there was no account to leave it against.
//
// The guard is the whole reason this is a method rather than two lines written twice.
// An address nobody has registered arrives here with an identifier of zero, and a
// write against zero is a write that names no row — which the store correctly refuses
// as "no such user", turning a plain wrong-address refusal into a failure. Answering
// early keeps the promise that an unregistered address leaves no trace and reads
// exactly like every other wrong pair.
func (userService *UserService) recordSignInOutcome(
	executionContext context.Context, userID uint, state vo.SignInLockoutStateVo,
) error {
	if userID == 0 {
		return nil
	}

	return userService.userRepository.SaveSignInLockoutState(executionContext, userID, state)
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
	storedSession, sessionExists, findError := userService.sessionHolding(
		executionContext, renewalDto.RefreshToken)
	if findError != nil {
		return dto.SessionTokensDto{}, findError
	}
	if !sessionExists {
		return dto.SessionTokensDto{}, domains.ErrAuthenticationRequired
	}

	session := domains.NewSessionDomain(storedSession)
	now := userService.clockProxy.Now()

	if session.Revoked() {
		return dto.SessionTokensDto{}, userService.tornDownChain(executionContext, session.ChainID())
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
	// Reading that the session was still good and writing to it are two moments, and
	// something can happen in between: a second renewal carrying the same proof, or a
	// sign-out. The store says so by refusing to rotate, and it means the same thing
	// the earlier check means — this proof has been used twice, so the chain goes.
	//
	// Without this, the second of two simultaneous renewals would quietly succeed and
	// leave two live sessions on one chain, and a rotation landing just after a
	// sign-out would put a working proof back into a chain somebody had just ended.
	if errors.Is(rotateError, domains.ErrSessionAlreadyRotated) {
		return dto.SessionTokensDto{}, userService.tornDownChain(executionContext, session.ChainID())
	}
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
	storedSession, sessionExists, findError := userService.sessionHolding(
		executionContext, renewalDto.RefreshToken)
	if findError != nil {
		return findError
	}
	if !sessionExists {
		return nil
	}

	// The whole chain goes, not just this session. A chain is one device's one
	// sign-in, and signing out means that device, not that proof.
	return userService.sessionRepository.RevokeChain(executionContext, storedSession.ChainID)
}

// tornDownChain ends every session of one sign-in and reports the refusal that goes
// with it, for the two places that discover a proof has been used twice — one by
// reading, one by failing to write.
//
// Tearing the chain down is the answer, so failing to tear it down is a failure of
// the request. Reporting "sign in again" while the second copy of the proof quietly
// still works would be the worst of both.
func (userService *UserService) tornDownChain(
	executionContext context.Context, chainID string,
) error {
	if revokeError := userService.sessionRepository.RevokeChain(
		executionContext, chainID); revokeError != nil {
		return revokeError
	}

	return domains.ErrAuthenticationRequired
}

// sessionHolding finds the session a renewal proof belongs to.
//
// The second return value says whether there was one, and it says only that — the
// two public methods that ask react to "there was not" in opposite ways. Renewing
// refuses; signing out succeeds, because a sign-in that is not there already does
// not work. A helper that decided for them would have to be told which of the two
// was calling, which is the same thing as not being a helper.
//
// A proof of nothing at all never reaches storage: there is nothing to look up, and
// asking would be a query whose answer is already known.
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

// IdentifyUser says who a proof of identity belongs to, whether or not they have
// been let in.
//
// It answers somebody still waiting rather than refusing them, and that is the whole
// reason it stays a separate question from the one the doors ask. Somebody waiting
// has exactly one way to find out they have been let in: look at themselves again.
// Refuse this and the only thing left to them is to keep signing in, which will
// never tell them anything.
func (userService *UserService) IdentifyUser(
	executionContext context.Context, accessToken string,
) (dto.UserDto, error) {
	user, identifyError := userService.identifiedUser(executionContext, accessToken)
	if identifyError != nil {
		return dto.UserDto{}, identifyError
	}

	return domains.NewAccountActivationDomain(user, userService.activationPolicy).ToUserDto(), nil
}

// IdentifyActivatedUser says who a proof of identity belongs to, and refuses unless
// they have been let in.
//
// This is the question every door asks, and it is one question on purpose. A door
// that asked who somebody was and then decided for itself whether that was good
// enough would be holding this feature's rule in the HTTP layer — and holding it once
// per door. Asked this way, the day "let in" grows a second meaning is a day the
// doors do not change.
//
// Recognising comes first and being let in comes second. The other order would answer
// a broken proof with "your account is not activated yet", which says something about
// an account nobody managed to identify.
func (userService *UserService) IdentifyActivatedUser(
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

// identifiedUser reads back the person a proof of identity was issued to.
//
// The user is read from the store rather than taken from the proof, and that is what
// makes a token stop working when the account behind it is gone — and what makes one
// start working the moment somebody is let in, with no need to sign in again. A proof
// carries who it was issued to, not what has become of them since; only the store
// knows that.
//
// It is private and shared by exactly the two public methods above, which is what
// earns it: one caller and it would belong inlined.
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

// ChangePassword replaces the password of the user this identifier names, and ends
// every session they have open.
//
// The order of the checks is chosen rather than incidental. Whether the new password
// is acceptable, and whether it merely repeats the old one, are questions that need
// no secret to answer and cost nothing to ask; whether the current password is the
// current password costs a bcrypt comparison, which is deliberately slow. Asking the
// cheap questions first means a request that simply filled a box in wrongly does not
// wait for the expensive one.
//
// The identifier does not come from anything the caller sent. It comes from the
// proof of identity the request carried, which is why there is no path here for
// changing somebody else's password: PasswordChangeDto has nowhere to name one.
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

	// No decoy work is spent when this fails, unlike signing in. There, refusing
	// faster than a real comparison would say "no account holds that address"; here
	// the person has already been recognised, so there is no list a timing
	// difference could describe.
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

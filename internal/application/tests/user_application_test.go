package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// signInMoment is the fixed clock reading so expiries are nameable values.
var signInMoment = time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)

// Expiries are written out rather than computed so tests assert the requirement, not the code's
// arithmetic.
var (
	accessTokenExpiry  = time.Date(2026, 9, 5, 8, 15, 0, 0, time.UTC)
	refreshTokenExpiry = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
)

var sessionLifetimes = vo.SessionLifetimesVo{
	AccessToken:  15 * time.Minute,
	RefreshToken: 30 * 24 * time.Hour,
}

// activationPolicy uses a stand-in mailbox; the tests only care that the subject follows the
// setting.
var activationPolicy = vo.AccountActivationPolicyVo{
	RequestMailbox: "gatekeeper@example.com",
	SubjectPrefix:  "console access request",
}

// lockoutPolicy uses the shipped defaults so "the third wrong password" means the same here.
var lockoutPolicy = vo.SignInLockoutPolicyVo{
	FailureThreshold: 3,
	LockoutDuration:  7 * 24 * time.Hour,
}

var lockoutExpiry = time.Date(2026, 9, 12, 8, 0, 0, 0, time.UTC)

// signInOutcomeRecorder captures the standing each sign-in writes, so tests assert values rather
// than calls and unrelated tests need no extra expectations.
type signInOutcomeRecorder struct {
	states  []vo.SignInLockoutStateVo
	userIDs []uint
	// observedCounts is the streak each write was guarded on.
	observedCounts []int
	failure        error
	// staleWrites is how many upcoming writes are refused as stale, simulating concurrent attempts.
	staleWrites int
}

func (recorder *signInOutcomeRecorder) only(t *testing.T) vo.SignInLockoutStateVo {
	t.Helper()
	require.Len(t, recorder.states, 1, "一次登入只該寫回一次狀態")

	return recorder.states[0]
}

type userApplicationUnderTest struct {
	userApplication    *application.UserApplication
	signInOutcome      *signInOutcomeRecorder
	userRepository     *mocks.MockIUserRepository
	sessionRepository  *mocks.MockISessionRepository
	passwordProofProxy *mocks.MockIPasswordProofProxy
	accessTokenProxy   *mocks.MockIAccessTokenProxy
	refreshTokenProxy  *mocks.MockIRefreshTokenProxy
	clockProxy         *mocks.MockIClockProxy
}

// newUserApplicationUnderTest uses the real domain service, mocking only stores, cryptographic
// proxies and the clock.
func newUserApplicationUnderTest(
	t *testing.T, lifetimes vo.SessionLifetimesVo,
) userApplicationUnderTest {
	mockController := gomock.NewController(t)
	userRepository := mocks.NewMockIUserRepository(mockController)
	sessionRepository := mocks.NewMockISessionRepository(mockController)
	passwordProofProxy := mocks.NewMockIPasswordProofProxy(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	refreshTokenProxy := mocks.NewMockIRefreshTokenProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(signInMoment).AnyTimes()

	signInOutcome := &signInOutcomeRecorder{}
	userRepository.EXPECT().
		SaveSignInLockoutState(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, userID uint, observedFailedSignInCount int,
			state vo.SignInLockoutStateVo,
		) error {
			if signInOutcome.failure != nil {
				return signInOutcome.failure
			}
			if signInOutcome.staleWrites > 0 {
				signInOutcome.staleWrites--

				return domains.ErrSignInLockoutStateStale
			}
			signInOutcome.userIDs = append(signInOutcome.userIDs, userID)
			signInOutcome.observedCounts = append(
				signInOutcome.observedCounts, observedFailedSignInCount)
			signInOutcome.states = append(signInOutcome.states, state)

			return nil
		}).AnyTimes()

	return userApplicationUnderTest{
		signInOutcome: signInOutcome,
		userApplication: application.NewUserApplication(
			service.NewUserService(
				userRepository, sessionRepository, passwordProofProxy,
				accessTokenProxy, refreshTokenProxy, clockProxy, lifetimes,
				activationPolicy, lockoutPolicy)),
		userRepository:     userRepository,
		sessionRepository:  sessionRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
		refreshTokenProxy:  refreshTokenProxy,
		clockProxy:         clockProxy,
	}
}

// The digest deliberately differs from the value, since the tests check which of the two travels
// where.
func aMintedRefreshToken() vo.RefreshTokenVo {
	return vo.RefreshTokenVo{Value: "a-refresh-token", Digest: "a-refresh-token-digest"}
}

func (fixture userApplicationUnderTest) expectSessionOpened() {
	fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
	fixture.accessTokenProxy.EXPECT().
		Issue(gomock.Any()).
		Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: accessTokenExpiry}, nil)
	fixture.sessionRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
			session.ID = 11
			return session, nil
		})
}

func aRegistrationDto() dto.UserRegistrationDto {
	return dto.UserRegistrationDto{Email: "James@Example.com", Password: "correct horse"}
}

func aStoredUser(id uint, email string) entities.User {
	return entities.User{
		ID:            id,
		Email:         email,
		PasswordProof: "a-password-proof",
		CreatedAt:     signInMoment,
		UpdatedAt:     signInMoment,
	}
}

func aLetInUser(id uint, email string) entities.User {
	user := aStoredUser(id, email)
	user.IsEnabled = true

	return user
}

func TestUserApplicationRegisterUser(t *testing.T) {
	t.Run("turns the password into a proof and stores what came back", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.passwordProofProxy.EXPECT().Prove("correct horse").Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, user entities.User) (entities.User, error) {
				assert.Equal(t, uint(0), user.ID, "還不存在的使用者不帶自己的識別碼")
				assert.Equal(t, "james@example.com", user.Email)
				assert.Equal(t, "a-password-proof", user.PasswordProof)

				return aStoredUser(7, user.Email), nil
			})

		userDto, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.NoError(t, err)
		assert.Equal(t, uint(7), userDto.ID)
		assert.Equal(t, "james@example.com", userDto.Email)
	})

	t.Run("nobody arrives already let in", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, user entities.User) (entities.User, error) {
				// Registration cannot set activation, so rows always arrive pending.
				assert.False(t, user.IsEnabled)

				return aStoredUser(7, user.Email), nil
			})

		userDto, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.NoError(t, err)
		assert.False(t, userDto.IsEnabled)
	})

	t.Run("says where to write and what to put in the subject", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, user entities.User) (entities.User, error) {
				return aStoredUser(7, user.Email), nil
			})

		// The subject must carry the normalised stored address, since that is what the inbox reader
		// matches on.
		userDto, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.NoError(t, err)
		require.NotNil(t, userDto.ActivationInstruction)
		assert.Equal(t, "gatekeeper@example.com", userDto.ActivationInstruction.RequestMailbox)
		assert.Equal(t,
			"console access request：james@example.com", userDto.ActivationInstruction.Subject)
	})

	t.Run("refuses an address that is not one without touching anything", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		registrationDto := aRegistrationDto()
		registrationDto.Email = "not-an-email"

		_, err := fixture.userApplication.RegisterUser(t.Context(), registrationDto)

		require.ErrorIs(t, err, domains.ErrUserValidation)
	})

	t.Run("refuses a password that breaks a rule without deriving anything", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		registrationDto := aRegistrationDto()
		registrationDto.Password = "short"

		_, err := fixture.userApplication.RegisterUser(t.Context(), registrationDto)

		require.ErrorIs(t, err, domains.ErrUserValidation)
	})

	t.Run("stores nothing when the proof cannot be derived", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		deriveFailure := errors.New("derive password proof: boom")
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("", deriveFailure)

		_, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.ErrorIs(t, err, deriveFailure)
	})

	t.Run("hands back an address somebody already holds as its own refusal", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.EmailAlreadyRegistered("james@example.com"))

		_, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.ErrorIs(t, err, domains.ErrEmailAlreadyRegistered)
		assert.Contains(t, err.Error(), "james@example.com")
	})

	t.Run("hands back a storage failure as itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("save user: connection closed")
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.ErrorIs(t, err, storageFailure)
	})
}

func aSignInDto() dto.SignInDto {
	return dto.SignInDto{Email: "　JAMES@Example.com　", Password: "correct horse"}
}

func TestUserApplicationSignIn(t *testing.T) {
	t.Run("hands back both halves of the session it opened", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), "james@example.com").
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().
			Matches("correct horse", "a-password-proof").
			Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: accessTokenExpiry}).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token", ExpiresAt: accessTokenExpiry}, nil)
		fixture.sessionRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
				session.ID = 11
				return session, nil
			})

		sessionTokensDto, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
		assert.Equal(t, "a-signed-token", sessionTokensDto.AccessToken)
		assert.Equal(t, accessTokenExpiry, sessionTokensDto.ExpiresAt)
		// The value goes to the holder and only the digest is stored.
		assert.Equal(t, "a-refresh-token", sessionTokensDto.RefreshToken)
		assert.Equal(t, refreshTokenExpiry, sessionTokensDto.RefreshTokenExpiresAt)
	})

	t.Run("lets in somebody who has not been let in yet", func(t *testing.T) {
		// Pending users still get a session so they can see their own standing.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
				session.ID = 11
				return session, nil
			})

		sessionTokensDto, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
		assert.Equal(t, "a-signed-token", sessionTokensDto.AccessToken)
	})

	t.Run("stores a session holding the digest, never the proof itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
				assert.Equal(t, uint(0), session.ID, "還不存在的登入階段不帶自己的識別碼")
				assert.Equal(t, uint(7), session.UserID)
				assert.Equal(t, "a-refresh-token-digest", session.RefreshTokenDigest)
				assert.NotEqual(t, "a-refresh-token", session.RefreshTokenDigest,
					"留存的必須是算不回去的那一份，不是續用憑證本身")
				assert.Equal(t, refreshTokenExpiry, session.ExpiresAt)
				assert.Nil(t, session.RevokedAt)
				assert.NotEmpty(t, session.ChainID)

				return session, nil
			})

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
	})

	t.Run("a shorter access token expires sooner", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, vo.SessionLifetimesVo{
			AccessToken: time.Hour, RefreshToken: 30 * 24 * time.Hour})
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC)}).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, session entities.Session) (entities.Session, error) {
				return session, nil
			})

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
	})

	t.Run("nothing is minted or stored when the pair does not match", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("a session that cannot be signed for leaves no session behind", func(t *testing.T) {
		// Signing before writing avoids storing a session whose proofs were never handed out.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrAccessTokenUnavailable)
	})

	t.Run("a proof that cannot be minted stores nothing and signs nothing", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		mintFailure := errors.New("mint refresh token: no randomness")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(vo.RefreshTokenVo{}, mintFailure)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, mintFailure)
	})

	t.Run("storage failing to open the session is not a wrong password", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("save session: connection closed")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, storageFailure)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("an address nobody holds is still put through the check", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.ErrUserNotFound)
		// The comparison still runs with no account, so response timing does not reveal whether the
		// address is registered.
		fixture.passwordProofProxy.EXPECT().Matches("correct horse", "").Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("both failures say exactly the same sentence", func(t *testing.T) {
		wrongPassword := newUserApplicationUnderTest(t, sessionLifetimes)
		wrongPassword.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		wrongPassword.passwordProofProxy.EXPECT().
			Matches(gomock.Any(), gomock.Any()).Return(false)

		noSuchAccount := newUserApplicationUnderTest(t, sessionLifetimes)
		noSuchAccount.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.ErrUserNotFound)
		noSuchAccount.passwordProofProxy.EXPECT().
			Matches(gomock.Any(), gomock.Any()).Return(false)

		_, wrongPasswordError := wrongPassword.userApplication.SignIn(t.Context(), aSignInDto())
		_, noSuchAccountError := noSuchAccount.userApplication.SignIn(t.Context(), aSignInDto())

		assert.Equal(t, "電子郵件或密碼不正確", wrongPasswordError.Error())
		assert.Equal(t, wrongPasswordError.Error(), noSuchAccountError.Error())
	})

	t.Run("a sign-in that cannot even be read never reaches storage", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		_, err := fixture.userApplication.SignIn(
			t.Context(), dto.SignInDto{Email: "not-an-email", Password: "correct horse"})

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("a password shorter than registering allows still signs in when it matches", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches("1234567", "a-password-proof").Return(true)
		fixture.expectSessionOpened()

		_, err := fixture.userApplication.SignIn(
			t.Context(), dto.SignInDto{Email: "james@example.com", Password: "1234567"})

		require.NoError(t, err, "長度規則管的是密碼設得成什麼，不是它現在對不對")
	})

	t.Run("storage being broken is not dressed up as a wrong password", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find user by email: connection closed")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected,
			"把系統壞掉說成密碼錯，會讓人一直重打一組本來就正確的密碼")
	})

}

func TestUserApplicationIdentifyUser(t *testing.T) {
	t.Run("says who a proof belongs to", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUser(7, "james@example.com"), nil)

		userDto, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.NoError(t, err)
		assert.Equal(t, uint(7), userDto.ID)
		assert.Equal(t, "james@example.com", userDto.Email)
	})

	t.Run("a proof that is not one never reaches storage", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy(gomock.Any()).
			Return(uint(0), domains.ErrAuthenticationRequired)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "a-tampered-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("an empty proof is not presented to the signing side at all", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a valid proof for somebody who is gone means signing in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, domains.ErrUserNotFound)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
		assert.Contains(t, err.Error(), "重新登入")
	})

	t.Run("storage being broken is not dressed up as an invalid proof", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find user: connection closed")
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("answers somebody still waiting, and says what to do about it", func(t *testing.T) {
		// This check deliberately does not require activation, so a pending user can learn they
		// were let in.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUser(7, "james@example.com"), nil)

		userDto, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.NoError(t, err)
		assert.False(t, userDto.IsEnabled)
		require.NotNil(t, userDto.ActivationInstruction)
		assert.Equal(t,
			"console access request：james@example.com", userDto.ActivationInstruction.Subject)
	})

	t.Run("stops saying it once they have been let in", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aLetInUser(7, "james@example.com"), nil)

		userDto, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.NoError(t, err)
		assert.True(t, userDto.IsEnabled)
		assert.Nil(t, userDto.ActivationInstruction)
	})
}

func TestUserApplicationIdentifyActivatedUser(t *testing.T) {
	t.Run("hands over somebody who has been let in", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aLetInUser(7, "james@example.com"), nil)

		userDto, err := fixture.userApplication.IdentifyActivatedUser(t.Context(), "a-signed-token")

		require.NoError(t, err)
		assert.Equal(t, uint(7), userDto.ID)
		assert.True(t, userDto.IsEnabled)
	})

	t.Run("refuses somebody still waiting, carrying what to do about it", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUser(7, "james@example.com"), nil)

		_, err := fixture.userApplication.IdentifyActivatedUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, domains.ErrAccountNotActivated)

		var notActivated domains.AccountNotActivatedError
		require.ErrorAs(t, err, &notActivated)
		assert.Equal(t, "gatekeeper@example.com", notActivated.Instruction.RequestMailbox)
		assert.Equal(t,
			"console access request：james@example.com", notActivated.Instruction.Subject)
	})

	t.Run("not being let in is never reported as needing to sign in", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUser(7, "james@example.com"), nil)

		_, err := fixture.userApplication.IdentifyActivatedUser(t.Context(), "a-signed-token")

		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a proof nobody can read is refused before activation is looked at", func(t *testing.T) {
		testCases := []struct {
			name        string
			accessToken string
			proofFails  bool
		}{
			{name: "nothing was presented", accessToken: ""},
			{name: "the proof was tampered with", accessToken: "a-tampered-token", proofFails: true},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newUserApplicationUnderTest(t, sessionLifetimes)
				if testCase.proofFails {
					fixture.accessTokenProxy.EXPECT().
						UserIdentifiedBy(gomock.Any()).
						Return(uint(0), domains.ErrAuthenticationRequired)
				}

				_, err := fixture.userApplication.IdentifyActivatedUser(
					t.Context(), testCase.accessToken)

				// Activation cannot be reported for an account that was never identified.
				require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
				assert.NotErrorIs(t, err, domains.ErrAccountNotActivated)
			})
		}
	})

	t.Run("a valid proof for somebody who is gone means signing in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, domains.ErrUserNotFound)

		_, err := fixture.userApplication.IdentifyActivatedUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
		assert.NotErrorIs(t, err, domains.ErrAccountNotActivated)
	})

	t.Run("storage being broken is not dressed up as either refusal", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find user: connection closed")
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.IdentifyActivatedUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
		assert.NotErrorIs(t, err, domains.ErrAccountNotActivated)
	})

	t.Run("the same proof starts working the moment somebody is let in", func(t *testing.T) {
		// Standing is read fresh each time rather than signed into the token, so activation applies
		// without re-sign-in.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy("a-signed-token").Return(uint(7), nil).Times(2)
		gomock.InOrder(
			fixture.userRepository.EXPECT().
				FindOne(gomock.Any(), uint(7)).
				Return(aStoredUser(7, "james@example.com"), nil),
			fixture.userRepository.EXPECT().
				FindOne(gomock.Any(), uint(7)).
				Return(aLetInUser(7, "james@example.com"), nil),
		)

		_, beforeError := fixture.userApplication.IdentifyActivatedUser(
			t.Context(), "a-signed-token")
		userDto, afterError := fixture.userApplication.IdentifyActivatedUser(
			t.Context(), "a-signed-token")

		require.ErrorIs(t, beforeError, domains.ErrAccountNotActivated)
		require.NoError(t, afterError)
		assert.True(t, userDto.IsEnabled)
	})
}

func TestUserApplicationIdentifyActivatedWebUser(t *testing.T) {
	testCases := []struct {
		name          string
		audience      string
		expectedError error
	}{
		{name: "a web sign-in is handed over", audience: ""},
		{name: "a connector's sign-in is refused as needing to sign in",
			audience: "https://mcp.example.com", expectedError: domains.ErrAuthenticationRequired},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newUserApplicationUnderTest(t, sessionLifetimes)
			fixture.accessTokenProxy.EXPECT().ClaimsOf("a-signed-token").Return(vo.AccessTokenClaimsVo{
				UserID: 7, Audience: testCase.audience, ExpiresAt: accessTokenExpiry,
			}, nil)
			fixture.accessTokenProxy.EXPECT().UserIdentifiedBy("a-signed-token").Return(uint(7), nil).AnyTimes()
			fixture.userRepository.EXPECT().FindOne(gomock.Any(), uint(7)).
				Return(aLetInUser(7, "james@example.com"), nil).AnyTimes()

			userDto, err := fixture.userApplication.IdentifyActivatedWebUser(t.Context(), "a-signed-token")

			if testCase.expectedError != nil {
				require.ErrorIs(t, err, testCase.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, uint(7), userDto.ID)
		})
	}

	t.Run("a proof nobody can read is refused", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.accessTokenProxy.EXPECT().ClaimsOf("a-tampered-token").
			Return(vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired)

		_, err := fixture.userApplication.IdentifyActivatedWebUser(t.Context(), "a-tampered-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})
}

func aStoredSession() entities.Session {
	return entities.Session{
		ID:                 11,
		UserID:             7,
		ChainID:            "a-chain",
		RefreshTokenDigest: "a-refresh-token-digest",
		ExpiresAt:          refreshTokenExpiry,
		CreatedAt:          signInMoment,
	}
}

func aRenewal() dto.SessionRenewalDto {
	return dto.SessionRenewalDto{RefreshToken: "a-refresh-token"}
}

func (fixture userApplicationUnderTest) expectDigestLookup() {
	fixture.refreshTokenProxy.EXPECT().
		DigestOf("a-refresh-token").
		Return("a-refresh-token-digest")
}

func TestUserApplicationRenewSession(t *testing.T) {
	t.Run("trades the proof for a fresh pair", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-refresh-token-digest").
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: accessTokenExpiry}).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token", ExpiresAt: accessTokenExpiry}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), uint(11), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, next entities.Session) (entities.Session, error) {
				next.ID = 12
				return next, nil
			})

		sessionTokensDto, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.NoError(t, err)
		assert.Equal(t, "a-newer-signed-token", sessionTokensDto.AccessToken)
		assert.Equal(t, "a-newer-token", sessionTokensDto.RefreshToken,
			"換回來的必須是新的那一份——回舊的等於這次換發沒有發生")
		assert.Equal(t, refreshTokenExpiry, sessionTokensDto.RefreshTokenExpiresAt)
	})

	t.Run("the successor stays on the same chain and starts its clock again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), uint(11), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, next entities.Session) (entities.Session, error) {
				assert.Equal(t, "a-chain", next.ChainID,
					"同一次登入換發下來的每一段共用一條鏈——換了鏈，登出就撤不乾淨")
				assert.Equal(t, uint(7), next.UserID)
				assert.Equal(t, "a-newer-digest", next.RefreshTokenDigest)
				assert.Equal(t, refreshTokenExpiry, next.ExpiresAt,
					"到期時刻從換發當下重算，不沿用舊的")
				assert.Nil(t, next.RevokedAt)
				assert.Equal(t, uint(0), next.ID)

				return next, nil
			})

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.NoError(t, err)
	})

	t.Run("a proof matching nothing is refused without tearing anything down", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionNotFound)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a proof that was already used tears down the whole chain", func(t *testing.T) {
		// A reused refresh token means two copies exist, so the whole session is ended for both
		// holders.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		revokedAt := signInMoment
		revokedSession := aStoredSession()
		revokedSession.RevokedAt = &revokedAt
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(revokedSession, nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("an expired proof is refused without tearing the chain down", func(t *testing.T) {
		// Expiry is not theft, so the session chain is left intact.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		expiredSession := aStoredSession()
		expiredSession.ExpiresAt = signInMoment.Add(-time.Second)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(expiredSession, nil)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a proof for somebody who is gone is refused", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, domains.ErrUserNotFound)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("every way of failing says exactly the same sentence", func(t *testing.T) {
		notFound := newUserApplicationUnderTest(t, sessionLifetimes)
		notFound.expectDigestLookup()
		notFound.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionNotFound)

		expired := newUserApplicationUnderTest(t, sessionLifetimes)
		expiredSession := aStoredSession()
		expiredSession.ExpiresAt = signInMoment.Add(-time.Second)
		expired.expectDigestLookup()
		expired.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(expiredSession, nil)

		gone := newUserApplicationUnderTest(t, sessionLifetimes)
		gone.expectDigestLookup()
		gone.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		gone.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.ErrUserNotFound)

		_, notFoundError := notFound.userApplication.RenewSession(t.Context(), aRenewal())
		_, expiredError := expired.userApplication.RenewSession(t.Context(), aRenewal())
		_, goneError := gone.userApplication.RenewSession(t.Context(), aRenewal())

		assert.Equal(t, "請重新登入", notFoundError.Error())
		assert.Equal(t, notFoundError.Error(), expiredError.Error())
		assert.Equal(t, notFoundError.Error(), goneError.Error())
	})

	t.Run("an empty proof never reaches storage", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		_, err := fixture.userApplication.RenewSession(t.Context(), dto.SessionRenewalDto{})

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("storage being broken is not a reason to sign in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find session: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, storageFailure)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("storage failing on the owner lookup is not a reason to sign in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find user: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("failing to tear down a stolen chain is reported, not swallowed", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		revokeFailure := errors.New("revoke session chain: connection closed")
		revokedAt := signInMoment
		revokedSession := aStoredSession()
		revokedSession.RevokedAt = &revokedAt
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(revokedSession, nil)
		fixture.sessionRepository.EXPECT().
			RevokeChain(gomock.Any(), gomock.Any()).
			Return(revokeFailure)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, revokeFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a rotation the store refuses tears the chain down", func(t *testing.T) {
		// A concurrent renewal or sign-out can land between read and write; the store reporting it
		// already ended counts as reuse.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionAlreadyRotated)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("failing to tear the chain down after a refused rotation is reported", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		revokeFailure := errors.New("revoke session chain: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionAlreadyRotated)
		fixture.sessionRepository.EXPECT().
			RevokeChain(gomock.Any(), gomock.Any()).
			Return(revokeFailure)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, revokeFailure)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a rotation that fails for any other reason leaves the chain alone", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		rotateFailure := errors.New("save renewed session: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token"}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.Session{}, rotateFailure)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, rotateFailure)
	})

	t.Run("a renewal that cannot be signed for rotates nothing", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any()).
			Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)

		_, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.ErrorIs(t, err, domains.ErrAccessTokenUnavailable)
	})
}

func TestUserApplicationRevokeSession(t *testing.T) {
	t.Run("ends the whole sign-in, not just the proof presented", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-refresh-token-digest").
			Return(aStoredSession(), nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()))
	})

	t.Run("a proof matching nothing is success, because the outcome is already true", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, domains.ErrSessionNotFound)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()))
	})

	t.Run("signing out twice is success both times", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		revokedAt := signInMoment
		revokedSession := aStoredSession()
		revokedSession.RevokedAt = &revokedAt
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(revokedSession, nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()))
	})

	t.Run("an expired proof still ends its sign-in", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		expiredSession := aStoredSession()
		expiredSession.ExpiresAt = signInMoment.Add(-time.Second)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(expiredSession, nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()))
	})

	t.Run("an empty proof never reaches storage", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), dto.SessionRenewalDto{}))
	})

	t.Run("storage being broken is reported", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("find session: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(entities.Session{}, storageFailure)

		require.ErrorIs(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()), storageFailure)
	})

	t.Run("failing to end the chain is reported", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		revokeFailure := errors.New("revoke session chain: connection closed")
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), gomock.Any()).
			Return(aStoredSession(), nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), gomock.Any()).Return(revokeFailure)

		require.ErrorIs(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()), revokeFailure)
	})
}

func TestUserApplicationChangePassword(t *testing.T) {
	const userID = uint(7)

	storedUser := entities.User{
		ID:            userID,
		Email:         "james@example.com",
		PasswordProof: "the-stored-proof",
	}

	t.Run("the right current password replaces the proof and ends every session", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), userID).Return(storedUser, nil)
		fixture.passwordProofProxy.EXPECT().
			Matches("correct horse", "the-stored-proof").
			Return(true)
		fixture.passwordProofProxy.EXPECT().
			Prove("battery staple").
			Return("the-new-proof", nil)
		fixture.userRepository.EXPECT().
			ChangePasswordProof(gomock.Any(), userID, "the-new-proof").
			Return(nil)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.NoError(t, err)
	})

	t.Run("the wrong current password is refused and nothing is written", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), userID).Return(storedUser, nil)
		fixture.passwordProofProxy.EXPECT().
			Matches("wrong horse", "the-stored-proof").
			Return(false)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "wrong horse", NewPassword: "battery staple"})

		// No Prove or ChangePasswordProof expectation is set, so reaching either fails the test.
		require.ErrorIs(t, err, domains.ErrCurrentPasswordRejected)
	})

	// Must differ from the sign-in refusal, since one leads back to sign-in and the other back to
	// the form.
	t.Run("a wrong current password is not the refusal a failed sign-in gives", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), userID).Return(storedUser, nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "wrong horse", NewPassword: "battery staple"})

		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected)
		assert.NotErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a user the identifier matches nobody is told to sign in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), userID).
			Return(entities.User{}, domains.ErrUserNotFound)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	// A storage failure must not be reported as a wrong password.
	t.Run("storage failing to answer is reported as itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		storageFailure := errors.New("the database is not there")
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), userID).
			Return(entities.User{}, storageFailure)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrCurrentPasswordRejected)
	})

	t.Run("a proof that cannot be derived fails the change", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		proveFailure := errors.New("the password is longer than the scheme reads")
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), userID).Return(storedUser, nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.passwordProofProxy.EXPECT().
			Prove("battery staple").
			Return("", proveFailure)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.ErrorIs(t, err, proveFailure)
	})

	t.Run("a write that fails is reported and leaves the change undone", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		writeFailure := errors.New("the transaction rolled back")
		fixture.userRepository.EXPECT().FindOne(gomock.Any(), userID).Return(storedUser, nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("the-new-proof", nil)
		fixture.userRepository.EXPECT().
			ChangePasswordProof(gomock.Any(), userID, "the-new-proof").
			Return(writeFailure)

		err := fixture.userApplication.ChangePassword(context.Background(), userID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.ErrorIs(t, err, writeFailure)
	})

	// Rule violations are refused before any read or comparison; no mocks are set up, so reaching
	// one fails.
	t.Run("an unacceptable new password is refused without touching anything", func(t *testing.T) {
		testCases := []struct {
			name            string
			newPassword     string
			expectedMessage string
		}{
			{name: "too short", newPassword: "1234567", expectedMessage: "密碼至少要 8 個字元"},
			{name: "empty", newPassword: "", expectedMessage: "必須給一組密碼"},
			{
				name:            "too long in bytes",
				newPassword:     strings.Repeat("密", 25),
				expectedMessage: "密碼長度上限為 72 個位元組",
			},
			{
				name:            "the same as the current one",
				newPassword:     "correct horse",
				expectedMessage: "新密碼不得與目前的密碼相同",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newUserApplicationUnderTest(t, sessionLifetimes)

				err := fixture.userApplication.ChangePassword(context.Background(), userID,
					dto.PasswordChangeDto{
						CurrentPassword: "correct horse",
						NewPassword:     testCase.newPassword,
					})

				require.ErrorIs(t, err, domains.ErrUserValidation)
				assert.Contains(t, err.Error(), testCase.expectedMessage)
			})
		}
	})

	// The user ID comes from the token, not from anything the caller sent.
	t.Run("the password changed is the one belonging to the identifier given", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		const otherUserID = uint(9)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), otherUserID).
			Return(entities.User{ID: otherUserID, PasswordProof: "another-proof"}, nil)
		fixture.passwordProofProxy.EXPECT().
			Matches("correct horse", "another-proof").
			Return(true)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("the-new-proof", nil)
		fixture.userRepository.EXPECT().
			ChangePasswordProof(gomock.Any(), otherUserID, "the-new-proof").
			Return(nil)

		err := fixture.userApplication.ChangePassword(context.Background(), otherUserID,
			dto.PasswordChangeDto{CurrentPassword: "correct horse", NewPassword: "battery staple"})

		require.NoError(t, err)
	})
}

func anAccountWithStanding(failedSignInCount int, lockedUntil *time.Time) entities.User {
	user := aStoredUser(7, "james@example.com")
	user.FailedSignInCount = failedSignInCount
	user.LockedUntil = lockedUntil

	return user
}

func shutUntilMoment(moment time.Time) *time.Time {
	return &moment
}

func TestUserApplicationSignInLockout(t *testing.T) {
	t.Run("a wrong password on a clean account is counted and nothing more", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 1, state.FailedSignInCount)
		assert.Nil(t, state.LockedUntil, "第一次猜錯還不該被鎖住")
	})

	t.Run("the third wrong password shuts the account for a week", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(2, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		// The attempt that reaches the threshold is itself refused.
		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 3, state.FailedSignInCount)
		require.NotNil(t, state.LockedUntil)
		assert.Equal(t, lockoutExpiry, *state.LockedUntil)
	})

	t.Run("getting the third one right shuts nothing and clears the streak", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(2, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.expectSessionOpened()

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 0, state.FailedSignInCount)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("a shut account is refused without its password being looked at", func(t *testing.T) {
		// No password proxy expectation is set: the lock is checked before the slow comparison, and
		// a locked account never reaches counting, so retries cannot extend the lock.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(3, shutUntilMoment(lockoutExpiry)), nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrSignInLocked)
		var locked domains.SignInLockedError
		require.ErrorAs(t, err, &locked)
		assert.Equal(t, lockoutExpiry, locked.LockedUntil)
		assert.Empty(t, fixture.signInOutcome.states, "鎖住期間不該再寫回任何狀態")
	})

	t.Run("the right password during the lock is refused just the same", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(3, shutUntilMoment(lockoutExpiry)), nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrSignInLocked)
		assert.Empty(t, fixture.signInOutcome.states)
	})

	t.Run("a lock ending exactly now lets the right password through", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(3, shutUntilMoment(signInMoment)), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.expectSessionOpened()

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 0, state.FailedSignInCount)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("the first wrong password after a served lock counts as one again", func(t *testing.T) {
		// Otherwise a user who waited out the lock would get one attempt back instead of three.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(3, shutUntilMoment(signInMoment.Add(-time.Second))), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 1, state.FailedSignInCount)
		assert.Nil(t, state.LockedUntil)
	})

	t.Run("an address nobody has registered is never shut and leaves nothing behind",
		func(t *testing.T) {
			// Still pays for the comparison so timing does not reveal whether the address exists.
			fixture := newUserApplicationUnderTest(t, sessionLifetimes)
			fixture.userRepository.EXPECT().
				FindOneByEmail(gomock.Any(), gomock.Any()).
				Return(entities.User{}, domains.ErrUserNotFound).
				Times(5)
			fixture.passwordProofProxy.EXPECT().
				Matches(gomock.Any(), gomock.Any()).
				Return(false).
				Times(5)

			for range 5 {
				_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

				require.ErrorIs(t, err, domains.ErrCredentialsRejected)
				require.NotErrorIs(t, err, domains.ErrSignInLocked,
					"不存在的電子郵件試幾次都不該變成被鎖住的那句話")
			}
			assert.Empty(t, fixture.signInOutcome.states,
				"沒有這個帳號，就沒有任何東西該被記下來")
		})

	t.Run("somebody nobody has let in is shut just the same", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		waiting := anAccountWithStanding(2, nil)
		waiting.IsEnabled = false
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(waiting, nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 3, state.FailedSignInCount)
		assert.NotNil(t, state.LockedUntil)
	})

	t.Run("a store that cannot remember the failure fails the sign-in", func(t *testing.T) {
		// Swallowing the error would silently disable the lock while the store is unwell.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.failure = errors.New("store is unwell")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(2, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected,
			"記不下來與密碼不對是兩件事，回同一句會讓這道鎖悄悄消失")
	})

	t.Run("a store that cannot remember the success hands out no session", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.failure = errors.New("store is unwell")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(2, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)

		sessionTokensDto, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.Error(t, err)
		assert.Empty(t, sessionTokensDto.AccessToken)
	})
}

func TestUserApplicationLockingAnAccountLeavesWhoeverIsAlreadyInsideAlone(t *testing.T) {
	// The lock only blocks new sign-ins, not existing sessions; these tests guard against someone
	// later making renewal check the lock.
	t.Run("a shut account still renews its proof of identity", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-refresh-token-digest").
			Return(aStoredSession(), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(anAccountWithStanding(3, shutUntilMoment(lockoutExpiry)), nil)
		fixture.refreshTokenProxy.EXPECT().
			Mint().
			Return(vo.RefreshTokenVo{Value: "a-newer-token", Digest: "a-newer-digest"}, nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: accessTokenExpiry}).
			Return(vo.AccessTokenVo{AccessToken: "a-newer-signed-token", ExpiresAt: accessTokenExpiry}, nil)
		fixture.sessionRepository.EXPECT().
			Rotate(gomock.Any(), uint(11), gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uint, next entities.Session) (entities.Session, error) {
				next.ID = 12
				return next, nil
			})

		sessionTokensDto, err := fixture.userApplication.RenewSession(t.Context(), aRenewal())

		require.NoError(t, err)
		assert.NotErrorIs(t, err, domains.ErrSignInLocked)
		assert.Equal(t, "a-newer-signed-token", sessionTokensDto.AccessToken)
	})

	t.Run("a shut account can still sign itself out", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.expectDigestLookup()
		fixture.sessionRepository.EXPECT().
			FindOneByDigest(gomock.Any(), "a-refresh-token-digest").
			Return(aStoredSession(), nil)
		fixture.sessionRepository.EXPECT().RevokeChain(gomock.Any(), "a-chain").Return(nil)

		require.NoError(t, fixture.userApplication.RevokeSession(t.Context(), aRenewal()))
	})
}

func TestUserApplicationSignInCountsEveryWrongPasswordWhenTheyArriveTogether(t *testing.T) {
	t.Run("a streak that moved underneath is counted against what it says now", func(t *testing.T) {
		// A concurrent attempt lands between reading the streak and writing it; overwriting with
		// the stale count would lose guesses.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 1
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(anAccountWithStanding(1, nil), nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 2, state.FailedSignInCount,
			"要從它現在真正的數字往上加，不是從我們讀到的那個")
		assert.Equal(t, []int{1}, fixture.signInOutcome.observedCounts,
			"寫入必須守在讀到的那個數字上，否則就是覆蓋別人的計數")
	})

	t.Run("the third of three arriving together still shuts the account", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 1
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(anAccountWithStanding(2, nil), nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		state := fixture.signInOutcome.only(t)
		assert.Equal(t, 3, state.FailedSignInCount)
		require.NotNil(t, state.LockedUntil, "並行進來的猜測一樣要觸發鎖定")
		assert.Equal(t, lockoutExpiry, *state.LockedUntil)
	})

	t.Run("an account somebody else just shut is left exactly as they shut it", func(t *testing.T) {
		// Counting on top would push the lock's end further out, letting anyone who knows an
		// address lock its owner out forever.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 1
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(2, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(anAccountWithStanding(3, shutUntilMoment(lockoutExpiry)), nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
		assert.Empty(t, fixture.signInOutcome.states,
			"已經被鎖住了就不再寫——再寫一次會把解除時刻往後推")
	})

	t.Run("a store that cannot be looked at again fails the sign-in", func(t *testing.T) {
		// If the re-read fails the guess is uncounted, so an error is returned rather than a plain
		// wrong-password refusal.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 1
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, errors.New("store is unwell"))

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("an attempt nobody could count fails the sign-in rather than vanishing", func(t *testing.T) {
		// Exhausting retries is an error, since an uncounted attempt would leave the lock short.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 99
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(anAccountWithStanding(1, nil), nil).
			AnyTimes()

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.Error(t, err)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("running out of looks against a shut account is not a failure", func(t *testing.T) {
		// Every lost round means another attempt was counted, so a locked account at the end
		// accounts for this attempt too.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.signInOutcome.staleWrites = 99
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(anAccountWithStanding(0, nil), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)
		gomock.InOrder(
			fixture.userRepository.EXPECT().
				FindOne(gomock.Any(), uint(7)).
				Return(anAccountWithStanding(1, nil), nil).Times(2),
			fixture.userRepository.EXPECT().
				FindOne(gomock.Any(), uint(7)).
				Return(anAccountWithStanding(3, shutUntilMoment(lockoutExpiry)), nil),
		)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected,
			"帳號已經被別人鎖住了，這一次就是一次單純的密碼錯誤")
	})
}

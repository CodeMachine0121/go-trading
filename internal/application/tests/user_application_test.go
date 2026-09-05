package application_test

import (
	"context"
	"errors"
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

// signInMoment is what the clock reads throughout these tests, so that "expires
// fifteen minutes from now" is a value the test can name rather than a moving target.
var signInMoment = time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)

// accessTokenExpiry and refreshTokenExpiry are signInMoment plus each lifetime,
// written out rather than computed so that a test asserting them is asserting the
// requirement and not repeating the code's arithmetic.
var (
	accessTokenExpiry  = time.Date(2026, 9, 5, 8, 15, 0, 0, time.UTC)
	refreshTokenExpiry = time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
)

var sessionLifetimes = vo.SessionLifetimesVo{
	AccessToken:  15 * time.Minute,
	RefreshToken: 30 * 24 * time.Hour,
}

type userApplicationUnderTest struct {
	userApplication    *application.UserApplication
	userRepository     *mocks.MockIUserRepository
	sessionRepository  *mocks.MockISessionRepository
	passwordProofProxy *mocks.MockIPasswordProofProxy
	accessTokenProxy   *mocks.MockIAccessTokenProxy
	refreshTokenProxy  *mocks.MockIRefreshTokenProxy
	clockProxy         *mocks.MockIClockProxy
}

// newUserApplicationUnderTest wires the real domain service and the real models,
// mocking only the outermost boundaries: the two stores, the three cryptographic
// capabilities, and the clock.
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

	return userApplicationUnderTest{
		userApplication: application.NewUserApplication(
			service.NewUserService(
				userRepository, sessionRepository, passwordProofProxy,
				accessTokenProxy, refreshTokenProxy, clockProxy, lifetimes)),
		userRepository:     userRepository,
		sessionRepository:  sessionRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
		refreshTokenProxy:  refreshTokenProxy,
		clockProxy:         clockProxy,
	}
}

// aMintedRefreshToken is what the minting capability hands back throughout these
// tests. The digest is deliberately unlike the value: everything that matters here
// turns on which of the two travels where.
func aMintedRefreshToken() vo.RefreshTokenVo {
	return vo.RefreshTokenVo{Value: "a-refresh-token", Digest: "a-refresh-token-digest"}
}

// expectSessionOpened sets up the three calls that opening a session always makes,
// for the tests that are about something else.
func (fixture userApplicationUnderTest) expectSessionOpened() {
	fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
	fixture.accessTokenProxy.EXPECT().
		Issue(gomock.Any(), gomock.Any()).
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
			Issue(uint(7), accessTokenExpiry).
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
		// The value goes to the holder; the digest stays behind. Handing out the
		// digest would give somebody something that opens nothing, and storing the
		// value would defeat the point of storing a digest at all.
		assert.Equal(t, "a-refresh-token", sessionTokensDto.RefreshToken)
		assert.Equal(t, refreshTokenExpiry, sessionTokensDto.RefreshTokenExpiresAt)
	})

	t.Run("stores a session holding the digest, never the proof itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
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
			Issue(uint(7), time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC)).
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
		// Signing before writing is what makes this true. The other order would end
		// with a stored session nobody was ever handed the proofs to.
		fixture := newUserApplicationUnderTest(t, sessionLifetimes)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.refreshTokenProxy.EXPECT().Mint().Return(aMintedRefreshToken(), nil)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
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
			Issue(gomock.Any(), gomock.Any()).
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
		// There is no account, so there is no proof — and the check still has to
		// happen. Turning back before it would answer measurably sooner than a wrong
		// password does, and how long the answer took is the same information as
		// whether the address is registered.
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
}

// aStoredSession is a session as it comes back from storage, good until the refresh
// token expiry unless a test says otherwise.
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

// expectDigestLookup sets up the derivation every renewal and sign-out starts with.
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
			Issue(uint(7), accessTokenExpiry).
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
			Issue(gomock.Any(), gomock.Any()).
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
		// A renewal proof works once. A used one turning up again means two copies
		// of it exist, and there is no way to tell which holder is the real one — so
		// the only safe answer is to end the sign-in for both.
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
		// Expiry is not theft. Tearing the chain down here would sign a second
		// device out for nothing.
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
		// Answering "sign in again" while the thief's copy quietly keeps working
		// would be the worst of both outcomes.
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
		// Reading that the session was good and writing to it are two moments. A
		// second renewal carrying the same proof, or a sign-out, can land between
		// them — and the store saying "this had already ended" means exactly what
		// finding an already-ended session means: the proof was used twice.
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
			Issue(gomock.Any(), gomock.Any()).
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
			Issue(gomock.Any(), gomock.Any()).
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
			Issue(gomock.Any(), gomock.Any()).
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
			Issue(gomock.Any(), gomock.Any()).
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

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

// signInMoment is what the clock reads throughout these tests, so that "expires a
// day from now" is a value the test can name rather than a moving target.
var signInMoment = time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)

const accessTokenLifetime = 24 * time.Hour

type userApplicationUnderTest struct {
	userApplication    *application.UserApplication
	userRepository     *mocks.MockIUserRepository
	passwordProofProxy *mocks.MockIPasswordProofProxy
	accessTokenProxy   *mocks.MockIAccessTokenProxy
}

// newUserApplicationUnderTest wires the real domain service and the real models,
// mocking only the outermost boundaries: storage, the two cryptographic capabilities,
// and the clock.
func newUserApplicationUnderTest(t *testing.T, lifetime time.Duration) userApplicationUnderTest {
	mockController := gomock.NewController(t)
	userRepository := mocks.NewMockIUserRepository(mockController)
	passwordProofProxy := mocks.NewMockIPasswordProofProxy(mockController)
	accessTokenProxy := mocks.NewMockIAccessTokenProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(signInMoment).AnyTimes()

	return userApplicationUnderTest{
		userApplication: application.NewUserApplication(
			service.NewUserService(
				userRepository, passwordProofProxy, accessTokenProxy, clockProxy, lifetime)),
		userRepository:     userRepository,
		passwordProofProxy: passwordProofProxy,
		accessTokenProxy:   accessTokenProxy,
	}
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
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
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
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)

		registrationDto := aRegistrationDto()
		registrationDto.Email = "not-an-email"

		_, err := fixture.userApplication.RegisterUser(t.Context(), registrationDto)

		require.ErrorIs(t, err, domains.ErrUserValidation)
	})

	t.Run("refuses a password that breaks a rule without deriving anything", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)

		registrationDto := aRegistrationDto()
		registrationDto.Password = "short"

		_, err := fixture.userApplication.RegisterUser(t.Context(), registrationDto)

		require.ErrorIs(t, err, domains.ErrUserValidation)
	})

	t.Run("stores nothing when the proof cannot be derived", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		deriveFailure := errors.New("derive password proof: boom")
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("", deriveFailure)

		_, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.ErrorIs(t, err, deriveFailure)
	})

	t.Run("hands back an address somebody already holds as its own refusal", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.passwordProofProxy.EXPECT().Prove(gomock.Any()).Return("a-password-proof", nil)
		fixture.userRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			Return(entities.User{}, domains.EmailAlreadyRegistered("james@example.com"))

		_, err := fixture.userApplication.RegisterUser(t.Context(), aRegistrationDto())

		require.ErrorIs(t, err, domains.ErrEmailAlreadyRegistered)
		assert.Contains(t, err.Error(), "james@example.com")
	})

	t.Run("hands back a storage failure as itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
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
	t.Run("issues a proof good for as long as a session lasts", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), "james@example.com").
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().
			Matches("correct horse", "a-password-proof").
			Return(true)
		fixture.accessTokenProxy.EXPECT().
			Issue(uint(7), time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC)).
			Return(vo.AccessTokenVo{
				AccessToken: "a-signed-token",
				ExpiresAt:   time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC),
			}, nil)

		accessTokenDto, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
		assert.Equal(t, "a-signed-token", accessTokenDto.AccessToken)
		assert.Equal(t, time.Date(2026, 9, 6, 8, 0, 0, 0, time.UTC), accessTokenDto.ExpiresAt)
	})

	t.Run("a shorter session expires sooner", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, time.Hour)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.accessTokenProxy.EXPECT().
			Issue(uint(7), time.Date(2026, 9, 5, 9, 0, 0, 0, time.UTC)).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.NoError(t, err)
	})

	t.Run("a wrong password issues nothing", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(false)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("an address nobody holds is still put through the check", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
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
		wrongPassword := newUserApplicationUnderTest(t, accessTokenLifetime)
		wrongPassword.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		wrongPassword.passwordProofProxy.EXPECT().
			Matches(gomock.Any(), gomock.Any()).Return(false)

		noSuchAccount := newUserApplicationUnderTest(t, accessTokenLifetime)
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
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)

		_, err := fixture.userApplication.SignIn(
			t.Context(), dto.SignInDto{Email: "not-an-email", Password: "correct horse"})

		require.ErrorIs(t, err, domains.ErrCredentialsRejected)
	})

	t.Run("a password shorter than registering allows still signs in when it matches", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches("1234567", "a-password-proof").Return(true)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
			Return(vo.AccessTokenVo{AccessToken: "a-signed-token"}, nil)

		_, err := fixture.userApplication.SignIn(
			t.Context(), dto.SignInDto{Email: "james@example.com", Password: "1234567"})

		require.NoError(t, err, "長度規則管的是密碼設得成什麼，不是它現在對不對")
	})

	t.Run("storage being broken is not dressed up as a wrong password", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		storageFailure := errors.New("find user by email: connection closed")
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(entities.User{}, storageFailure)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrCredentialsRejected,
			"把系統壞掉說成密碼錯，會讓人一直重打一組本來就正確的密碼")
	})

	t.Run("hands back a system that cannot sign as itself", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.userRepository.EXPECT().
			FindOneByEmail(gomock.Any(), gomock.Any()).
			Return(aStoredUser(7, "james@example.com"), nil)
		fixture.passwordProofProxy.EXPECT().Matches(gomock.Any(), gomock.Any()).Return(true)
		fixture.accessTokenProxy.EXPECT().
			Issue(gomock.Any(), gomock.Any()).
			Return(vo.AccessTokenVo{}, domains.ErrAccessTokenUnavailable)

		_, err := fixture.userApplication.SignIn(t.Context(), aSignInDto())

		require.ErrorIs(t, err, domains.ErrAccessTokenUnavailable)
	})
}

func TestUserApplicationIdentifyUser(t *testing.T) {
	t.Run("says who a proof belongs to", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
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
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.accessTokenProxy.EXPECT().
			UserIdentifiedBy(gomock.Any()).
			Return(uint(0), domains.ErrAuthenticationRequired)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "a-tampered-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("an empty proof is not presented to the signing side at all", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
	})

	t.Run("a valid proof for somebody who is gone means signing in again", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
		fixture.accessTokenProxy.EXPECT().UserIdentifiedBy(gomock.Any()).Return(uint(7), nil)
		fixture.userRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).
			Return(entities.User{}, domains.ErrUserNotFound)

		_, err := fixture.userApplication.IdentifyUser(t.Context(), "a-signed-token")

		require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
		assert.Contains(t, err.Error(), "重新登入")
	})

	t.Run("storage being broken is not dressed up as an invalid proof", func(t *testing.T) {
		fixture := newUserApplicationUnderTest(t, accessTokenLifetime)
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

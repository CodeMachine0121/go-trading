package persistence_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func userWithEmail(email string) entities.User {
	return entities.User{Email: email, PasswordProof: "a-password-proof"}
}

func TestUserRepositorySaveHandsBackTheUserAsStored(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))

	require.NoError(t, saveError)
	assert.Positive(t, savedUser.ID)
	assert.False(t, savedUser.CreatedAt.IsZero())
	assert.Equal(t, "james@example.com", savedUser.Email)
}

func TestUserRepositorySaveRefusesAnAddressAlreadyHeld(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	_, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	_, conflictError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))

	require.ErrorIs(t, conflictError, domains.ErrEmailAlreadyRegistered)
	assert.Contains(t, conflictError.Error(), "james@example.com")

	_, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError, "被拒絕的那一次不得留下任何東西，既有那一位也不得被動到")
}

func TestUserRepositoryFindOneByEmail(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	foundUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")

	require.NoError(t, findError)
	assert.Equal(t, savedUser.ID, foundUser.ID)
	assert.Equal(t, "a-password-proof", foundUser.PasswordProof,
		"留存的證明得原樣讀得回來，否則沒有人登得進去")
}

func TestUserRepositoryFindOneByEmailSaysNothingIsThere(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	_, findError := userRepository.FindOneByEmail(t.Context(), "nobody@example.com")

	require.ErrorIs(t, findError, domains.ErrUserNotFound)
}

func TestUserRepositoryFindOne(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	foundUser, findError := userRepository.FindOne(t.Context(), savedUser.ID)

	require.NoError(t, findError)
	assert.Equal(t, "james@example.com", foundUser.Email)
}

func TestUserRepositoryFindOneSaysNothingIsThere(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	_, findError := userRepository.FindOne(t.Context(), 99999)

	require.ErrorIs(t, findError, domains.ErrUserNotFound)
}

func TestUserRepositorySaysStorageBrokeRatherThanAnsweringWithNothing(t *testing.T) {
	userRepository := persistence.NewUserRepository(closedDatabase(t))

	_, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	_, findByEmailError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	_, findError := userRepository.FindOne(t.Context(), 1)

	require.Error(t, saveError)
	assert.NotErrorIs(t, saveError, domains.ErrEmailAlreadyRegistered)
	require.Error(t, findByEmailError)
	assert.NotErrorIs(t, findByEmailError, domains.ErrUserNotFound,
		"連不上資料庫不等於查無此人——那會讓人以為自己的帳號被刪了")
	require.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrUserNotFound)

	changeError := userRepository.ChangePasswordProof(t.Context(), 1, "the-new-proof")
	require.Error(t, changeError)
	assert.NotErrorIs(t, changeError, domains.ErrUserNotFound,
		"連不上資料庫不等於查無此人——那會讓人以為自己的帳號被刪了")
}

// The index name is repeated because struct tags cannot hold constants; this test needs no database, so it never skips.
func TestTheEmailIndexTheRepositoryBlamesIsTheOneTheEntityDeclares(t *testing.T) {
	emailField, found := reflect.TypeFor[entities.User]().FieldByName("Email")
	require.True(t, found, "the entity has no Email field to carry the index")

	assert.Contains(t, emailField.Tag.Get("gorm"), "uniqueIndex:"+persistence.UserEmailIndex)
}

func TestUserRepositorySaveBlamesTheAddressOnlyWhenTheAddressIsWhatClashed(t *testing.T) {
	// A primary-key clash (e.g. a stale sequence after a restore) must not be reported as a taken address.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	firstUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	clashingIdentifier := userWithEmail("someone-else@example.com")
	clashingIdentifier.ID = firstUser.ID

	_, clashError := userRepository.Save(t.Context(), clashingIdentifier)

	require.Error(t, clashError)
	assert.NotErrorIs(t, clashError, domains.ErrEmailAlreadyRegistered,
		"撞到的是識別碼，不是電子郵件——說錯了會害人去找一個根本不存在的帳號")
}

func TestUserRepositoryFindOneByEmailRefusesToGuessWhenGivenNothing(t *testing.T) {
	// GORM drops zero-valued struct fields, so an empty address must not become no condition at all.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	_, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	_, findError := userRepository.FindOneByEmail(t.Context(), "")

	require.ErrorIs(t, findError, domains.ErrUserNotFound)
}

func TestUserRepositoryChangePasswordProofReplacesTheProof(t *testing.T) {
	database := newTestDatabase(t)
	userRepository := persistence.NewUserRepository(database)
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	changeError := userRepository.ChangePasswordProof(t.Context(), savedUser.ID, "the-new-proof")

	require.NoError(t, changeError)
	reloadedUser, findError := userRepository.FindOne(t.Context(), savedUser.ID)
	require.NoError(t, findError)
	assert.Equal(t, "the-new-proof", reloadedUser.PasswordProof)
}

func TestUserRepositoryChangePasswordProofEndsEverySessionThatUserHasOpen(t *testing.T) {
	database := newTestDatabase(t)
	userRepository := persistence.NewUserRepository(database)
	sessionRepository := persistence.NewSessionRepository(database)
	owner, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	laptop, laptopError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "laptop-chain", "laptop-digest"))
	require.NoError(t, laptopError)
	phone, phoneError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "phone-chain", "phone-digest"))
	require.NoError(t, phoneError)

	require.NoError(t, userRepository.ChangePasswordProof(t.Context(), owner.ID, "the-new-proof"))

	reloadedLaptop, laptopFindError := sessionRepository.FindOneByDigest(t.Context(), "laptop-digest")
	require.NoError(t, laptopFindError)
	assert.NotNil(t, reloadedLaptop.RevokedAt, "換完密碼，發動變更的那一台也得跟著失效")
	assert.Equal(t, laptop.ID, reloadedLaptop.ID)

	reloadedPhone, phoneFindError := sessionRepository.FindOneByDigest(t.Context(), "phone-digest")
	require.NoError(t, phoneFindError)
	assert.NotNil(t, reloadedPhone.RevokedAt, "另一台裝置也得跟著失效")
	assert.Equal(t, phone.ID, reloadedPhone.ID)
}

func TestUserRepositoryChangePasswordProofLeavesEverybodyElseAlone(t *testing.T) {
	database := newTestDatabase(t)
	userRepository := persistence.NewUserRepository(database)
	sessionRepository := persistence.NewSessionRepository(database)
	changer, changerError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, changerError)
	bystander, bystanderError := userRepository.Save(t.Context(), userWithEmail("someone@example.com"))
	require.NoError(t, bystanderError)
	_, sessionError := sessionRepository.Save(
		t.Context(), sessionOf(bystander.ID, "bystander-chain", "bystander-digest"))
	require.NoError(t, sessionError)

	require.NoError(t, userRepository.ChangePasswordProof(t.Context(), changer.ID, "the-new-proof"))

	reloadedBystander, findError := userRepository.FindOne(t.Context(), bystander.ID)
	require.NoError(t, findError)
	assert.Equal(t, "a-password-proof", reloadedBystander.PasswordProof)

	reloadedSession, sessionFindError := sessionRepository.FindOneByDigest(
		t.Context(), "bystander-digest")
	require.NoError(t, sessionFindError)
	assert.Nil(t, reloadedSession.RevokedAt, "別人的登入階段不該因為這一次變更而失效")
}

func TestUserRepositoryChangePasswordProofLeavesAlreadyEndedSessionsAsTheyWere(t *testing.T) {
	database := newTestDatabase(t)
	userRepository := persistence.NewUserRepository(database)
	sessionRepository := persistence.NewSessionRepository(database)
	owner, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)
	_, sessionError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "old-chain", "old-digest"))
	require.NoError(t, sessionError)
	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "old-chain"))

	endedBefore, beforeError := sessionRepository.FindOneByDigest(t.Context(), "old-digest")
	require.NoError(t, beforeError)
	require.NotNil(t, endedBefore.RevokedAt)

	require.NoError(t, userRepository.ChangePasswordProof(t.Context(), owner.ID, "the-new-proof"))

	endedAfter, afterError := sessionRepository.FindOneByDigest(t.Context(), "old-digest")
	require.NoError(t, afterError)
	require.NotNil(t, endedAfter.RevokedAt)
	assert.Equal(t, endedBefore.RevokedAt.UTC(), endedAfter.RevokedAt.UTC())
}

func TestUserRepositoryChangePasswordProofSaysNobodyIsThere(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	changeError := userRepository.ChangePasswordProof(t.Context(), 9999, "the-new-proof")

	require.ErrorIs(t, changeError, domains.ErrUserNotFound)
}

// An unstorable proof is a storage failure, not a missing user, and the transaction must leave the password unchanged.
func TestUserRepositoryChangePasswordProofSaysStorageBrokeRatherThanBlamingTheUser(t *testing.T) {
	database := newTestDatabase(t)
	userRepository := persistence.NewUserRepository(database)
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	changeError := userRepository.ChangePasswordProof(
		t.Context(), savedUser.ID, strings.Repeat("x", 256))

	require.Error(t, changeError)
	assert.NotErrorIs(t, changeError, domains.ErrUserNotFound)

	reloadedUser, findError := userRepository.FindOne(t.Context(), savedUser.ID)
	require.NoError(t, findError)
	assert.Equal(t, "a-password-proof", reloadedUser.PasswordProof,
		"寫失敗的那一次不得改動任何東西")
}

func TestUserRepositoryStartsEverybodyWithNothingHeldAgainstThem(t *testing.T) {
	// Column defaults give existing rows a clean lockout record without a backfill.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))

	require.NoError(t, saveError)
	assert.Equal(t, 0, savedUser.FailedSignInCount)
	assert.Nil(t, savedUser.LockedUntil)
}

func TestUserRepositorySavesAndClearsWhatAnAttemptLeftBehind(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)
	shutUntil := time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)

	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 3, LockedUntil: &shutUntil}))

	shutUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError)
	assert.Equal(t, 3, shutUser.FailedSignInCount)
	require.NotNil(t, shutUser.LockedUntil)
	assert.Equal(t, shutUntil, shutUser.LockedUntil.UTC())

	// Clearing must reset both columns.
	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID, 3,
		vo.SignInLockoutStateVo{FailedSignInCount: 0, LockedUntil: nil}))

	clearedUser, refindError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, refindError)
	assert.Equal(t, 0, clearedUser.FailedSignInCount)
	assert.Nil(t, clearedUser.LockedUntil)
}

func TestUserRepositoryRefusesToRecordAnAttemptAgainstNobody(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	recordError := userRepository.SaveSignInLockoutState(t.Context(), 4242, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 1})

	require.ErrorIs(t, recordError, domains.ErrUserNotFound)
}

func TestUserRepositoryChangingThePasswordAlsoOpensTheDoor(t *testing.T) {
	// A password change requires a valid identity proof, so it also lifts the lock.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)
	shutUntil := time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)
	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 3, LockedUntil: &shutUntil}))

	require.NoError(t, userRepository.ChangePasswordProof(
		t.Context(), savedUser.ID, "a-new-password-proof"))

	changedUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError)
	assert.Equal(t, "a-new-password-proof", changedUser.PasswordProof)
	assert.Equal(t, 0, changedUser.FailedSignInCount)
	assert.Nil(t, changedUser.LockedUntil)
}

func TestUserRepositorySaysSoWhenTheStoreCannotRecordAnAttempt(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	cancelledContext, cancel := context.WithCancel(t.Context())
	cancel()

	recordError := userRepository.SaveSignInLockoutState(cancelledContext, savedUser.ID, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 1})

	require.Error(t, recordError)
	assert.NotErrorIs(t, recordError, domains.ErrUserNotFound,
		"寫不進去與查無此人是兩件事")
}

func TestUserRepositoryRefusesAWriteCountedFromAStreakThatHasSinceMoved(t *testing.T) {
	// Two attempts reading the same streak must not both write, or parallel guesses would cost a single increment.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 1}))

	staleError := userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID, 0,
		vo.SignInLockoutStateVo{FailedSignInCount: 1})

	require.ErrorIs(t, staleError, domains.ErrSignInLockoutStateStale)
	assert.NotErrorIs(t, staleError, domains.ErrUserNotFound,
		"這一列還在，只是變了——與查無此人是兩件事，上游對它們的反應不同")

	unchangedUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError)
	assert.Equal(t, 1, unchangedUser.FailedSignInCount, "被拒絕的那一次不得改動任何東西")
}

func TestUserRepositoryLosesNoAttemptWhenManyArriveAtOnce(t *testing.T) {
	// Every concurrent attempt must be counted against the streak it actually read.
	const attemptCount = 100

	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	var attempts sync.WaitGroup
	var counted atomic.Int64
	for range attemptCount {
		attempts.Add(1)
		go func() {
			defer attempts.Done()
			// Each attempt retries until its own increment lands, as the sign-in flow does.
			for {
				current, findError := userRepository.FindOne(t.Context(), savedUser.ID)
				if findError != nil {
					return
				}
				writeError := userRepository.SaveSignInLockoutState(
					t.Context(), savedUser.ID, current.FailedSignInCount,
					vo.SignInLockoutStateVo{FailedSignInCount: current.FailedSignInCount + 1})
				if writeError == nil {
					counted.Add(1)

					return
				}
				if !errors.Is(writeError, domains.ErrSignInLockoutStateStale) {
					return
				}
			}
		}()
	}
	attempts.Wait()

	finalUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError)
	assert.Equal(t, int64(attemptCount), counted.Load(), "每一次嘗試都要被算到")
	assert.Equal(t, attemptCount, finalUser.FailedSignInCount,
		"同時打進來的猜測不得互相覆蓋——否則一百次猜測只花掉一次")
}

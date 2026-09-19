package persistence_test

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// userWithEmail is a user who differs from their siblings only by address, so that a
// test about addresses is not also a test about anything else.
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

// The repository names the index it blames in Go; the entity spells it in a struct
// tag, which cannot hold a constant. Nothing but this stops the two drifting, and if
// they drift a taken address stops being answered as a conflict and starts being
// answered as a storage failure. This test needs no database, so unlike the ones
// above it cannot skip.
func TestTheEmailIndexTheRepositoryBlamesIsTheOneTheEntityDeclares(t *testing.T) {
	emailField, found := reflect.TypeFor[entities.User]().FieldByName("Email")
	require.True(t, found, "the entity has no Email field to carry the index")

	assert.Contains(t, emailField.Tag.Get("gorm"), "uniqueIndex:"+persistence.UserEmailIndex)
}

func TestUserRepositorySaveBlamesTheAddressOnlyWhenTheAddressIsWhatClashed(t *testing.T) {
	// Every table has more than one thing that can clash. The identifier is the
	// obvious other one, and it clashes for a reason nobody's choice of address had
	// any part in — a restored dump that left the identifier sequence behind. Saying
	// "somebody has that address" there would send whoever reads it hunting for an
	// account that does not exist.
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
	// GORM drops zero-valued struct fields, so a struct-form condition on an empty
	// address becomes no condition at all — and this would hand back whichever user
	// is first in the table, whose stored proof would then be checked against
	// somebody's typed password. Nothing reaches here with an empty address today;
	// this is so that nothing can start to.
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

// This is the whole reason the two writes are one method. A password changed while
// an old session keeps working is exactly the situation somebody changes their
// password to end.
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

// Somebody else's sign-ins are not this person's business, and neither is their
// password.
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

// A session that had already ended keeps the moment it ended. Overwriting it would
// erase the only trail there is to when the sign-in actually stopped.
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

// A change aimed at nobody must not quietly succeed, and must not sign anybody out
// on its way to finding that out.
func TestUserRepositoryChangePasswordProofSaysNobodyIsThere(t *testing.T) {
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	changeError := userRepository.ChangePasswordProof(t.Context(), 9999, "the-new-proof")

	require.ErrorIs(t, changeError, domains.ErrUserNotFound)
}

// A proof the column cannot hold is a storage failure, not a missing user. Saying
// "no such user" for it would send whoever reads it looking for an account that is
// sitting right there — and because the write happens inside a transaction, the
// refusal must also leave the password exactly as it was.
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
	// The column defaults are what make this safe to add to a table that already has
	// rows: everybody who was here before this lock existed begins with a clean
	// record, rather than needing a one-off script somebody has to remember to run.
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

	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID,
		vo.SignInLockoutStateVo{FailedSignInCount: 3, LockedUntil: &shutUntil}))

	shutUser, findError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, findError)
	assert.Equal(t, 3, shutUser.FailedSignInCount)
	require.NotNil(t, shutUser.LockedUntil)
	assert.Equal(t, shutUntil, shutUser.LockedUntil.UTC())

	// Clearing has to reach both columns. A count left at three beside an absent
	// moment would shut the account again on the very next mistake.
	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID,
		vo.SignInLockoutStateVo{FailedSignInCount: 0, LockedUntil: nil}))

	clearedUser, refindError := userRepository.FindOneByEmail(t.Context(), "james@example.com")
	require.NoError(t, refindError)
	assert.Equal(t, 0, clearedUser.FailedSignInCount)
	assert.Nil(t, clearedUser.LockedUntil)
}

func TestUserRepositoryRefusesToRecordAnAttemptAgainstNobody(t *testing.T) {
	// Quietly writing nothing would mean the lock silently does not exist for that
	// account, and the sign-in flow that asked would carry on believing it does.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))

	recordError := userRepository.SaveSignInLockoutState(t.Context(), 4242,
		vo.SignInLockoutStateVo{FailedSignInCount: 1})

	require.ErrorIs(t, recordError, domains.ErrUserNotFound)
}

func TestUserRepositoryChangingThePasswordAlsoOpensTheDoor(t *testing.T) {
	// Somebody changing their password needed a valid proof of identity to get
	// this far, so they have already proved they are the account holder. Keeping
	// them shut out afterwards protects nothing.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)
	shutUntil := time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)
	require.NoError(t, userRepository.SaveSignInLockoutState(t.Context(), savedUser.ID,
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
	// A store that cannot answer is not the same as an account that does not exist,
	// and the sign-in flow acts differently on each: one is a failed sign-in, the
	// other is an address nobody holds. Returning the storage failure as itself is
	// what keeps them apart.
	userRepository := persistence.NewUserRepository(newTestDatabase(t))
	savedUser, saveError := userRepository.Save(t.Context(), userWithEmail("james@example.com"))
	require.NoError(t, saveError)

	cancelledContext, cancel := context.WithCancel(t.Context())
	cancel()

	recordError := userRepository.SaveSignInLockoutState(cancelledContext, savedUser.ID,
		vo.SignInLockoutStateVo{FailedSignInCount: 1})

	require.Error(t, recordError)
	assert.NotErrorIs(t, recordError, domains.ErrUserNotFound,
		"寫不進去與查無此人是兩件事")
}

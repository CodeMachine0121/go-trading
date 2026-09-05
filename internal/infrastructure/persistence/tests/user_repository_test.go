package persistence_test

import (
	"reflect"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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

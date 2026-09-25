package entities_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserToDtoHandsOutWhoTheyAreAndNothingElse(t *testing.T) {
	user := entities.User{
		ID:            7,
		Email:         "james@example.com",
		PasswordProof: "$2a$12$XnhfeGHwjLbM/cah350NkOeZnpiIZUnm8UF4w3HoxjbuZbxdkrzl6",
		IsEnabled:     true,
	}

	userDto := user.ToDto()

	assert.Equal(t, uint(7), userDto.ID)
	assert.Equal(t, "james@example.com", userDto.Email)
	assert.True(t, userDto.IsEnabled)
}

func TestUserToDtoCarriesStillWaitingToBeLetIn(t *testing.T) {
	user := entities.User{ID: 7, Email: "james@example.com"}

	userDto := user.ToDto()

	assert.False(t, userDto.IsEnabled)
	assert.Nil(t, userDto.ActivationInstruction)
}

func TestUserDtoCarriesNoTraceOfThePasswordProof(t *testing.T) {
	user := entities.User{
		ID:            7,
		Email:         "james@example.com",
		PasswordProof: "$2a$12$XnhfeGHwjLbM/cah350NkOeZnpiIZUnm8UF4w3HoxjbuZbxdkrzl6",
	}

	// Check the serialised JSON, since that is what actually leaves the system.
	encodedUserDto, err := json.Marshal(user.ToDto())

	require.NoError(t, err)
	assert.JSONEq(t,
		`{"id":7,"email":"james@example.com","isEnabled":false}`, string(encodedUserDto))
}

func TestUserIsStoredInItsOwnTable(t *testing.T) {
	assert.Equal(t, "Users", entities.User{}.TableName())
}

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
	}

	userDto := user.ToDto()

	assert.Equal(t, uint(7), userDto.ID)
	assert.Equal(t, "james@example.com", userDto.Email)
}

func TestUserDtoCarriesNoTraceOfThePasswordProof(t *testing.T) {
	user := entities.User{
		ID:            7,
		Email:         "james@example.com",
		PasswordProof: "$2a$12$XnhfeGHwjLbM/cah350NkOeZnpiIZUnm8UF4w3HoxjbuZbxdkrzl6",
	}

	// Serialised is how the answer actually leaves the system, so it is the honest
	// place to check that the proof is not in it — a field nobody reads is still a
	// field the JSON carries.
	encodedUserDto, err := json.Marshal(user.ToDto())

	require.NoError(t, err)
	assert.JSONEq(t, `{"id":7,"email":"james@example.com"}`, string(encodedUserDto))
}

func TestUserIsStoredInItsOwnTable(t *testing.T) {
	assert.Equal(t, "Users", entities.User{}.TableName())
}

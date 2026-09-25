package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var gatekeeping = vo.AccountActivationPolicyVo{
	RequestMailbox: "gatekeeper@example.com",
	SubjectPrefix:  "console access request",
}

func TestAccountActivationTellsWhetherSomebodyIsStillWaiting(t *testing.T) {
	testCases := []struct {
		name            string
		isEnabled       bool
		expectedPending bool
	}{
		{name: "somebody nobody has let in is waiting", isEnabled: false, expectedPending: true},
		{name: "somebody who was let in is not waiting", isEnabled: true, expectedPending: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			activation := domains.NewAccountActivationDomain(
				entities.User{ID: 7, Email: "alice@example.com", IsEnabled: testCase.isEnabled},
				gatekeeping,
			)

			assert.Equal(t, testCase.expectedPending, activation.Pending())
		})
	}
}

func TestAccountActivationTellsAWaitingPersonWhereToWriteAndWhatToPutInTheSubject(t *testing.T) {
	activation := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping)

	userDto := activation.ToUserDto()

	assert.False(t, userDto.IsEnabled)
	require.NotNil(t, userDto.ActivationInstruction)
	assert.Equal(t, "gatekeeper@example.com", userDto.ActivationInstruction.RequestMailbox)
	// The inbox reader matches a letter to an account by this exact string.
	assert.Equal(t,
		"console access request：alice@example.com", userDto.ActivationInstruction.Subject)
}

func TestAccountActivationNamesTheAddressTheAccountIsActuallyUnder(t *testing.T) {
	// The subject must carry the stored spelling, not what was typed.
	activation := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping)

	userDto := activation.ToUserDto()

	require.NotNil(t, userDto.ActivationInstruction)
	assert.NotContains(t, userDto.ActivationInstruction.Subject, "Alice@Example.com")
	assert.Contains(t, userDto.ActivationInstruction.Subject, "alice@example.com")
}

func TestAccountActivationDropsTheInstructionOnceSomebodyIsLetIn(t *testing.T) {
	activation := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com", IsEnabled: true}, gatekeeping)

	userDto := activation.ToUserDto()

	assert.True(t, userDto.IsEnabled)
	assert.Nil(t, userDto.ActivationInstruction)
}

func TestAccountActivationRefusalCarriesTheSameInstruction(t *testing.T) {
	activation := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping)

	refusal := activation.NotActivatedError()

	require.ErrorIs(t, refusal, domains.ErrAccountNotActivated)

	var notActivated domains.AccountNotActivatedError
	require.ErrorAs(t, refusal, &notActivated)
	assert.Equal(t, "gatekeeper@example.com", notActivated.Instruction.RequestMailbox)
	assert.Equal(t, "console access request：alice@example.com", notActivated.Instruction.Subject)
}

func TestNotBeingLetInIsNotTheSameRefusalAsNotBeingRecognised(t *testing.T) {
	// Distinct refusals because they call for opposite actions: a pending user who signs in again lands in the same place.
	refusal := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping).NotActivatedError()

	assert.NotErrorIs(t, refusal, domains.ErrAuthenticationRequired)
}

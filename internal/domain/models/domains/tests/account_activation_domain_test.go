package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatekeeping is a stand-in for wherever the person running this console reads their
// mail. What the tests below are about is how the subject line is put together, not
// which inbox it lands in.
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
	// Assembled, not handed over in pieces: the person reading the inbox matches a
	// letter to an account by this exact string.
	assert.Equal(t,
		"console access request：alice@example.com", userDto.ActivationInstruction.Subject)
}

func TestAccountActivationNamesTheAddressTheAccountIsActuallyUnder(t *testing.T) {
	// Whatever was typed on the day, the row holds the one spelling the account is
	// known by — and the subject has to carry that one, or it matches nobody.
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
	// An instruction that no longer applies is worse than none: somebody would
	// follow it.
	assert.Nil(t, userDto.ActivationInstruction)
}

func TestAccountActivationRefusalCarriesTheSameInstruction(t *testing.T) {
	activation := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping)

	refusal := activation.NotActivatedError()

	// Recognisable without reading the sentence, because that is what lets a caller
	// act on it.
	require.ErrorIs(t, refusal, domains.ErrAccountNotActivated)

	var notActivated domains.AccountNotActivatedError
	require.ErrorAs(t, refusal, &notActivated)
	assert.Equal(t, "gatekeeper@example.com", notActivated.Instruction.RequestMailbox)
	assert.Equal(t, "console access request：alice@example.com", notActivated.Instruction.Subject)
}

func TestNotBeingLetInIsNotTheSameRefusalAsNotBeingRecognised(t *testing.T) {
	// The two call for opposite actions. Told to sign in again, a person who is
	// merely waiting would sign in successfully and land in exactly the same place.
	refusal := domains.NewAccountActivationDomain(
		entities.User{ID: 7, Email: "alice@example.com"}, gatekeeping).NotActivatedError()

	assert.NotErrorIs(t, refusal, domains.ErrAuthenticationRequired)
}

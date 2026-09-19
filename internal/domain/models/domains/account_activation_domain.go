package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AccountActivationDomain is one person's standing with the door: whether they have
// been let in, and — while they have not — exactly what they have to do about it.
//
// Everything this feature knows lives here and nowhere else: what "waiting" means,
// how the subject line is put together, and whether the instruction belongs in an
// answer at all. The door upstream asks a question and gets an answer; it does not
// hold a boolean and decide for itself. That is what makes the next state — suspended,
// expired, let in for a trial — a change to this file rather than to every handler
// standing behind the door.
type AccountActivationDomain struct {
	user   entities.User
	policy vo.AccountActivationPolicyVo
}

// NewAccountActivationDomain reads one person's standing from the row that holds it
// and the policy that says where to write.
//
// It is built here rather than by a method on the user, because the models this
// package works with already come from entities — a conversion in the other direction
// would be a cycle, not a preference.
func NewAccountActivationDomain(
	user entities.User, policy vo.AccountActivationPolicyVo,
) AccountActivationDomain {
	return AccountActivationDomain{user: user, policy: policy}
}

// Pending is whether this person is still waiting to be let in.
func (accountActivationDomain AccountActivationDomain) Pending() bool {
	return !accountActivationDomain.user.IsEnabled
}

// ToUserDto is this person as they leave the domain, carrying the instruction only
// while it still applies.
//
// It builds on the row's own conversion rather than restating the fields, so "what a
// user looks like from outside" keeps having exactly one home.
func (accountActivationDomain AccountActivationDomain) ToUserDto() dto.UserDto {
	userDto := accountActivationDomain.user.ToDto()

	if accountActivationDomain.Pending() {
		instruction := accountActivationDomain.instruction()
		userDto.ActivationInstruction = &instruction
	}

	return userDto
}

// NotActivatedError is the refusal this person gets from every door, with the
// instruction attached so that the door never has to know where letters go.
func (accountActivationDomain AccountActivationDomain) NotActivatedError() error {
	return AccountNotActivatedError{Instruction: accountActivationDomain.instruction()}
}

// instruction is where to write and what to write in the subject line.
//
// The address in the subject is the one that was stored — already trimmed and
// lowered — rather than whatever spelling was typed on the day. The subject is how
// the person reading the inbox matches a letter to an account, so it has to be the
// spelling the account is actually under.
//
// It is private and shared by the two public methods above, which is the only reason
// it is a method at all rather than written twice.
func (accountActivationDomain AccountActivationDomain) instruction() dto.AccountActivationInstructionDto {
	return dto.AccountActivationInstructionDto{
		RequestMailbox: accountActivationDomain.policy.RequestMailbox,
		Subject: fmt.Sprintf("%s：%s",
			accountActivationDomain.policy.SubjectPrefix, accountActivationDomain.user.Email),
	}
}

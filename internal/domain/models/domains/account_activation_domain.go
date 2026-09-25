package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AccountActivationDomain decides whether a person is still waiting for activation and what instruction to give them, so callers never branch on the raw flag.
type AccountActivationDomain struct {
	user   entities.User
	policy vo.AccountActivationPolicyVo
}

func NewAccountActivationDomain(
	user entities.User, policy vo.AccountActivationPolicyVo,
) AccountActivationDomain {
	return AccountActivationDomain{user: user, policy: policy}
}

func (accountActivationDomain AccountActivationDomain) Pending() bool {
	return !accountActivationDomain.user.IsEnabled
}

// ToUserDto attaches the activation instruction only while the account is pending.
func (accountActivationDomain AccountActivationDomain) ToUserDto() dto.UserDto {
	userDto := accountActivationDomain.user.ToDto()

	if accountActivationDomain.Pending() {
		instruction := accountActivationDomain.instruction()
		userDto.ActivationInstruction = &instruction
	}

	return userDto
}

// NotActivatedError carries the instruction so callers never need to know where requests go.
func (accountActivationDomain AccountActivationDomain) NotActivatedError() error {
	return AccountNotActivatedError{Instruction: accountActivationDomain.instruction()}
}

// instruction uses the stored (normalised) email in the subject so the inbox reader can match it to the account.
func (accountActivationDomain AccountActivationDomain) instruction() dto.AccountActivationInstructionDto {
	return dto.AccountActivationInstructionDto{
		RequestMailbox: accountActivationDomain.policy.RequestMailbox,
		Subject: fmt.Sprintf("%s：%s",
			accountActivationDomain.policy.SubjectPrefix, accountActivationDomain.user.Email),
	}
}

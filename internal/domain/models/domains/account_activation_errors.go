package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ErrAccountNotActivated is deliberately distinct from ErrAuthenticationRequired, which sends people back to sign in, something that cannot help here.
var ErrAccountNotActivated = errors.New("帳號尚未開通，請寄信申請開通")

// AccountNotActivatedError carries the activation instruction to the HTTP layer without it knowing the mailbox setting.
type AccountNotActivatedError struct {
	Instruction dto.AccountActivationInstructionDto
}

func (accountNotActivatedError AccountNotActivatedError) Error() string {
	return ErrAccountNotActivated.Error()
}

// Is lets callers match with errors.Is while errors.As still reaches the instruction.
func (accountNotActivatedError AccountNotActivatedError) Is(target error) bool {
	return target == ErrAccountNotActivated
}

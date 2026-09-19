package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ErrAccountNotActivated is what every door says to somebody it recognises but has
// not been told to let in.
//
// It is deliberately not ErrAuthenticationRequired. In this system that one means a
// single thing — "this sign-in no longer counts, go and sign in again" — and callers
// act on it by taking the person back to the sign-in screen. This person's sign-in is
// fine; sending them back there would have them do the one thing that cannot possibly
// change their situation.
var ErrAccountNotActivated = errors.New("帳號尚未開通，請寄信申請開通")

// AccountNotActivatedError is that refusal carrying what to do about it.
//
// The instruction travels inside the error because the door is the place that has to
// say it and the last place that should know it: where letters go is a setting, and a
// setting that reached the HTTP layer would be a second copy of a decision the domain
// already made.
type AccountNotActivatedError struct {
	Instruction dto.AccountActivationInstructionDto
}

func (accountNotActivatedError AccountNotActivatedError) Error() string {
	return ErrAccountNotActivated.Error()
}

// Is lets callers that only want to know which refusal this is keep writing
// errors.Is, while the ones that need the instruction reach for errors.As.
func (accountNotActivatedError AccountNotActivatedError) Is(target error) bool {
	return target == ErrAccountNotActivated
}

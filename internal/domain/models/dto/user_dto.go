package dto

// UserDto is the only shape a user leaves the domain in: who they are, what they
// sign in as, and whether they have been let in yet.
//
// Neither the password nor the proof derived from it has a place here, so no caller
// can hand either of them onwards by accident and no future field can quietly add
// one back without somebody having to write it down.
type UserDto struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	// IsEnabled says whether this person may actually use the system. It travels
	// with every answer rather than being asked for separately, because the one
	// thing somebody waiting to be let in needs is a way to find out that they have
	// been — and the only way they have is to look at themselves again.
	IsEnabled bool `json:"isEnabled"`
	// ActivationInstruction is what to do about not being let in yet, and it is
	// present only while that is true.
	//
	// It is a pointer so that it disappears from the answer entirely once the person
	// is in, rather than arriving as an empty shell the caller has to decide whether
	// to believe. An instruction that no longer applies is worse than no instruction:
	// somebody would follow it.
	ActivationInstruction *AccountActivationInstructionDto `json:"activationInstruction,omitempty"`
}

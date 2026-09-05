package dto

// UserDto is the only shape a user leaves the domain in: who they are and what they
// sign in as.
//
// It has exactly two fields, and the absence of a third is the design. Neither the
// password nor the proof derived from it has a place here, so no caller can hand
// either of them onwards by accident and no future field can quietly add one back
// without somebody having to write it down.
type UserDto struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
}

package dto

// AccountActivationInstructionDto is what somebody waiting to be let in has to do
// about it: write to this address, with this subject.
//
// The subject arrives assembled rather than as a prefix and an address to join up.
// Joining them is one line, and one line written in the browser, in the connector and
// in whatever comes next is three chances to join them differently — and a subject
// that came out differently is a subject the person reading the inbox cannot match to
// anybody.
type AccountActivationInstructionDto struct {
	RequestMailbox string `json:"requestMailbox"`
	Subject        string `json:"subject"`
}

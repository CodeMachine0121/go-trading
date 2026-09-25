package dto

// AccountActivationInstructionDto carries the subject fully assembled so every client sends
// an identical, matchable subject line.
type AccountActivationInstructionDto struct {
	RequestMailbox string `json:"requestMailbox"`
	Subject        string `json:"subject"`
}

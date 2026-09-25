package dto

// RequesterDto is what a request says about who sent it; the domain decides whom it counts against.
type RequesterDto struct {
	// AccessToken is empty when none was sent.
	AccessToken   string
	ClientAddress string
}

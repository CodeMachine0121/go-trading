package vo

// RefreshTokenVo is one freshly minted renewal proof: the value handed to whoever
// will use it, and the digest kept in its place. Immutable, no behavior.
//
// The two travel together because the value exists for exactly one moment. After it
// has been handed out there is no way back to it from what was stored — that is the
// point — so anything that needs both has to be given both at once.
type RefreshTokenVo struct {
	Value  string
	Digest string
}

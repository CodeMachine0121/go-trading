package vo

// OpaqueIdentifierVo pairs a freshly minted identifier with its digest; the identifier cannot be recovered from the digest afterwards.
type OpaqueIdentifierVo struct {
	Value  string
	Digest string
}

package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_opaque_identifier_proxy.go -destination=mocks/mock_i_opaque_identifier_proxy.go -package=mocks

// IOpaqueIdentifierProxy mints unguessable identifiers such as client ids, request ids and authorization codes; refresh tokens stay with IRefreshTokenProxy.
type IOpaqueIdentifierProxy interface {
	// Mint returns the identifier together with a digest, for identifiers that must be stored only as a digest.
	Mint() (vo.OpaqueIdentifierVo, error)
	// DigestOf is deterministic so an identifier stored as a digest can be looked up.
	DigestOf(identifier string) string
}

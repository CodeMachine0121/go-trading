package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// SessionTokensVo is one pair of proofs as they leave the domain: the one every
// request carries, the one that renews it, and when the renewal proof stops working.
//
// It is one value rather than two loose halves because a caller must never be able
// to hand out one without the other. A holder given only an access token has fifteen
// minutes and then no way back; a holder given only a renewal proof cannot make a
// single request.
type SessionTokensVo struct {
	AccessToken  AccessTokenVo
	RefreshToken RefreshTokenVo
	// RefreshTokenExpiresAt is taken from the session as it was actually stored,
	// not worked out a second time alongside it. Two calculations of one moment are
	// two chances to disagree, and the one that reaches the holder would be the one
	// nothing enforces.
	RefreshTokenExpiresAt time.Time
}

// ToDto is this pair in the shape the domain hands outwards. Both moments are always
// handed out in universal time.
func (sessionTokensVo SessionTokensVo) ToDto() dto.SessionTokensDto {
	return dto.SessionTokensDto{
		AccessToken:           sessionTokensVo.AccessToken.AccessToken,
		ExpiresAt:             sessionTokensVo.AccessToken.ExpiresAt.UTC(),
		RefreshToken:          sessionTokensVo.RefreshToken.Value,
		RefreshTokenExpiresAt: sessionTokensVo.RefreshTokenExpiresAt.UTC(),
	}
}

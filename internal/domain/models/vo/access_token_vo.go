package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// AccessTokenVo is one issued proof of identity, as the signing side hands it back:
// the token and the moment it expires. Immutable, no behavior.
//
// It exists so that whoever issues a token cannot hand back only the token and
// leave the expiry to be guessed at — the two are one answer, and separating them
// is how "when does this stop working" becomes nobody's job to say.
type AccessTokenVo struct {
	AccessToken string
	ExpiresAt   time.Time
}

// ToDto is this token in the shape the domain hands outwards. The expiry is always
// handed out in universal time, whatever zone it was built in.
func (accessTokenVo AccessTokenVo) ToDto() dto.AccessTokenDto {
	return dto.AccessTokenDto{
		AccessToken: accessTokenVo.AccessToken,
		ExpiresAt:   accessTokenVo.ExpiresAt.UTC(),
	}
}

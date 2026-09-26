package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// SessionTokensVo is the access and refresh token pair, kept as one value so neither can be handed out alone.
type SessionTokensVo struct {
	AccessToken  AccessTokenVo
	RefreshToken RefreshTokenVo
	// RefreshTokenExpiresAt is taken from the stored session rather than recomputed.
	RefreshTokenExpiresAt time.Time
}

// ToDto hands both moments out in UTC.
func (sessionTokensVo SessionTokensVo) ToDto() dto.SessionTokensDto {
	return dto.SessionTokensDto{
		AccessToken:           sessionTokensVo.AccessToken.AccessToken,
		ExpiresAt:             sessionTokensVo.AccessToken.ExpiresAt.UTC(),
		RefreshToken:          sessionTokensVo.RefreshToken.Value,
		RefreshTokenExpiresAt: sessionTokensVo.RefreshTokenExpiresAt.UTC(),
	}
}

func (sessionTokensVo SessionTokensVo) ToConnectorTokensDto(now time.Time) dto.ConnectorTokensDto {
	return dto.ConnectorTokensDto{
		AccessToken:      sessionTokensVo.AccessToken.AccessToken,
		TokenType:        "Bearer",
		ExpiresInSeconds: int64(sessionTokensVo.AccessToken.ExpiresAt.Sub(now).Seconds()),
		RefreshToken:     sessionTokensVo.RefreshToken.Value,
	}
}

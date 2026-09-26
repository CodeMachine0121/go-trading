package vo

import (
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// AccessTokenClaimsVo is what an access token asserts; Audience is empty for web sign-ins.
type AccessTokenClaimsVo struct {
	UserID    uint
	Audience  string
	ExpiresAt time.Time
}

func (accessTokenClaimsVo AccessTokenClaimsVo) ToIntrospectionDto() dto.AccessTokenIntrospectionDto {
	subject := strconv.FormatUint(uint64(accessTokenClaimsVo.UserID), 10)
	audience := accessTokenClaimsVo.Audience
	expiresAt := accessTokenClaimsVo.ExpiresAt.Unix()

	return dto.AccessTokenIntrospectionDto{
		Active:    true,
		Subject:   &subject,
		Audience:  &audience,
		ExpiresAt: &expiresAt,
	}
}

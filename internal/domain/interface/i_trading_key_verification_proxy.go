package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_trading_key_verification_proxy.go -destination=mocks/mock_i_trading_key_verification_proxy.go -package=mocks

// ITradingKeyVerificationProxy asks an exchange whether a trading key is accepted and what it may trade; Binance is currently the only implementation.
type ITradingKeyVerificationProxy interface {
	// VerifyTradingKey returns the exchange's refusal as a reason with a nil error; only local failures are errors, and none may carry either key string.
	VerifyTradingKey(
		executionContext context.Context, credential vo.TradingKeyCredentialVo,
	) (vo.TradingKeyVerificationVo, error)
}

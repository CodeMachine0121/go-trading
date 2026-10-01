package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ErrBinanceTradingKeyValidation wraps a message naming the broken rule of a trading key.
var ErrBinanceTradingKeyValidation = errors.New("binance trading key validation failed")

// ErrBinanceTradingKeyNotConfigured is a normal state for reads, never shown as a failure.
var ErrBinanceTradingKeyNotConfigured = errors.New("尚未完成幣安交易金鑰設定")

// ErrBinanceTradingKeySealUnavailable is the system's failure, not the caller's; storing unsealed is never the fallback.
var ErrBinanceTradingKeySealUnavailable = errors.New("系統目前無法安全保存幣安交易金鑰")

// ErrBinanceTradingKeyVerificationFailed matches every BinanceTradingKeyVerificationError.
var ErrBinanceTradingKeyVerificationFailed = errors.New("binance trading key verification failed")

// BinanceTradingKeyVerificationError carries the named reason to the HTTP layer so callers can tell the four refusals apart without parsing text.
type BinanceTradingKeyVerificationError struct {
	Reason vo.TradingKeyVerificationFailureVo
}

func (verificationError BinanceTradingKeyVerificationError) Error() string {
	switch verificationError.Reason {
	case vo.TradingKeyVerificationFailureKeyRejected:
		return "幣安不接受這組金鑰，請確認 API Key 與 Secret Key 後重新填寫"
	case vo.TradingKeyVerificationFailureTimedOut:
		return "等幣安回答等太久，這次沒有存成，請稍後再試"
	case vo.TradingKeyVerificationFailureNoTradingPermission:
		return "這組幣安交易金鑰沒有任何交易權限，請到幣安開啟現貨或合約交易權限"
	default:
		return "連不上幣安，請稍後再試"
	}
}

func (verificationError BinanceTradingKeyVerificationError) Is(target error) bool {
	return target == ErrBinanceTradingKeyVerificationFailed
}

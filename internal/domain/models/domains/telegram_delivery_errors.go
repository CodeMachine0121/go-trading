package domains

import "errors"

// ErrTelegramDeliveryValidation wraps a message naming the broken rule of a delivery setting or message.
var ErrTelegramDeliveryValidation = errors.New("telegram delivery validation failed")

// ErrTelegramDeliveryNotConfigured is deliberately not a validation failure: the fix is setting delivery up, not correcting input.
var ErrTelegramDeliveryNotConfigured = errors.New("尚未完成 Telegram 設定")

// ErrSecretSealUnavailable means no sealing key is configured; storing is refused rather than keeping the token unsealed.
var ErrSecretSealUnavailable = errors.New("系統目前無法安全保存機器人金鑰")

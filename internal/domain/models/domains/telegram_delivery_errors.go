package domains

import "errors"

// ErrTelegramDeliveryValidation marks a delivery setting, or a message, whose
// content broke one of its rules. The wrapped message names the rule, so a caller
// reports it without knowing the list.
var ErrTelegramDeliveryValidation = errors.New("telegram delivery validation failed")

// ErrTelegramDeliveryNotConfigured marks somebody asking to send when they have not
// said where to send.
//
// It is deliberately not a validation failure: nothing they typed is wrong, and the
// thing they must do about it is different — go and set the delivery up, rather than
// fix the box in front of them.
var ErrTelegramDeliveryNotConfigured = errors.New("尚未完成 Telegram 設定")

// ErrSecretSealUnavailable marks a system that cannot lock away a secret, because it
// has nothing to lock it with. It is not a rejection of anything the caller sent.
//
// It is a separate failure from every other because the alternative — storing the
// token unlocked — is a system that looks like it is protecting the token and is
// not. Nobody would find out until the day somebody read the table, and by then
// every token in it is spent. Refusing to store is the only safe way to be missing
// the key, exactly as refusing to sign is the only safe way to be missing a signing
// key.
var ErrSecretSealUnavailable = errors.New("系統目前無法安全保存機器人金鑰")

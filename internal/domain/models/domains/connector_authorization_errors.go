package domains

import "errors"

var ErrConnectorRedirectUriInvalid = errors.New("送回地址只能是本機的 http 位址")

var ErrConnectorClientMetadataInvalid = errors.New("外掛登記內容不合格")

var ErrConnectorClientNotFound = errors.New("外掛不存在")

// ErrConnectorRedirectUriNotRegistered is never answered with a redirect, since the address itself is untrusted.
var ErrConnectorRedirectUriNotRegistered = errors.New("送回地址不是這個外掛登記過的")

// ErrConnectorAuthorizationRequestNotFound covers unknown, expired and already decided alike.
var ErrConnectorAuthorizationRequestNotFound = errors.New("這筆外掛授權請求不存在、已過期或已經決定過")

var ErrConnectorAuthorizationCodeNotFound = errors.New("authorization code not found")

// ErrConnectorAuthorizationCodeAlreadyRedeemed reports the write-time single-use check; callers treat it as reuse.
var ErrConnectorAuthorizationCodeAlreadyRedeemed = errors.New("authorization code already redeemed")

var ErrConnectorTokenRequestInvalid = errors.New("換授權的請求缺少必要資料或格式不合格")

// ErrConnectorGrantInvalid covers every refused code alike so nothing about the code is revealed.
var ErrConnectorGrantInvalid = errors.New("授權碼無效")

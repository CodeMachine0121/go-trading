package vo

// TradingKeyCredentialVo is the only type holding a whole secret key; it lives for one hop from domain to infrastructure and never travels back out.
type TradingKeyCredentialVo struct {
	ApiKey    string
	SecretKey string
}

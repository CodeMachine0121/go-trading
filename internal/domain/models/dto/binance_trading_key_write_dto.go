package dto

// BinanceTradingKeyWriteDto is inbound only; the owner comes from the request's credentials.
type BinanceTradingKeyWriteDto struct {
	ApiKey    string
	SecretKey string
}

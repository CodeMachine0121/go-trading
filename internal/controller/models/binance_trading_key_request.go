package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// BinanceTradingKeyRequest names no account, since the account comes from the sign-in.
type BinanceTradingKeyRequest struct {
	ApiKey    string `json:"apiKey"`
	SecretKey string `json:"secretKey"`
}

func (binanceTradingKeyRequest BinanceTradingKeyRequest) ToBinanceTradingKeyWriteDto() dto.BinanceTradingKeyWriteDto {
	return dto.BinanceTradingKeyWriteDto{
		ApiKey:    binanceTradingKeyRequest.ApiKey,
		SecretKey: binanceTradingKeyRequest.SecretKey,
	}
}

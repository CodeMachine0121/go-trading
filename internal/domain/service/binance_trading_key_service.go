package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// BinanceTradingKeyService is the application layer's only entry point for trading keys; nothing in it ever unseals one.
type BinanceTradingKeyService struct {
	binanceTradingKeyRepository domaininterface.IBinanceTradingKeyRepository
	secretSealProxy             domaininterface.ISecretSealProxy
	tradingKeyVerificationProxy domaininterface.ITradingKeyVerificationProxy
}

func NewBinanceTradingKeyService(
	binanceTradingKeyRepository domaininterface.IBinanceTradingKeyRepository,
	secretSealProxy domaininterface.ISecretSealProxy,
	tradingKeyVerificationProxy domaininterface.ITradingKeyVerificationProxy,
) *BinanceTradingKeyService {
	return &BinanceTradingKeyService{
		binanceTradingKeyRepository: binanceTradingKeyRepository,
		secretSealProxy:             secretSealProxy,
		tradingKeyVerificationProxy: tradingKeyVerificationProxy,
	}
}

// GetTradingKey returns the owner's view with the API key tail, or Configured=false when none is stored.
func (binanceTradingKeyService *BinanceTradingKeyService) GetTradingKey(
	executionContext context.Context, userID uint,
) (dto.BinanceTradingKeyDto, error) {
	binanceTradingKey, findError := binanceTradingKeyService.binanceTradingKeyRepository.FindOneByUser(
		executionContext, userID)
	if errors.Is(findError, domains.ErrBinanceTradingKeyNotConfigured) {
		return dto.BinanceTradingKeyDto{Configured: false, TradableMarkets: []string{}}, nil
	}
	if findError != nil {
		return dto.BinanceTradingKeyDto{}, findError
	}

	return binanceTradingKey.ToDto(), nil
}

// GetTradingKeyStatus returns only whether a key is stored and what it may trade, for readers such as connectors that must see no part of the key.
func (binanceTradingKeyService *BinanceTradingKeyService) GetTradingKeyStatus(
	executionContext context.Context, userID uint,
) (dto.BinanceTradingKeyStatusDto, error) {
	binanceTradingKey, findError := binanceTradingKeyService.binanceTradingKeyRepository.FindOneByUser(
		executionContext, userID)
	if errors.Is(findError, domains.ErrBinanceTradingKeyNotConfigured) {
		return dto.BinanceTradingKeyStatusDto{Configured: false, TradableMarkets: []string{}}, nil
	}
	if findError != nil {
		return dto.BinanceTradingKeyStatusDto{}, findError
	}

	return binanceTradingKey.ToStatusDto(), nil
}

// SaveTradingKey seals before asking Binance so a system that cannot store safely refuses without a network round trip; any refusal leaves the stored key and every switch untouched.
func (binanceTradingKeyService *BinanceTradingKeyService) SaveTradingKey(
	executionContext context.Context, userID uint, writeDto dto.BinanceTradingKeyWriteDto,
) (dto.BinanceTradingKeyDto, error) {
	tradingKey, validationError := domains.NewBinanceTradingKeyDomain(writeDto)
	if validationError != nil {
		return dto.BinanceTradingKeyDto{}, validationError
	}

	credential := tradingKey.ToCredentialVo()

	sealedApiKey, sealApiKeyError := binanceTradingKeyService.secretSealProxy.Seal(credential.ApiKey)
	sealedSecretKey, sealSecretKeyError := binanceTradingKeyService.secretSealProxy.Seal(credential.SecretKey)
	if sealError := errors.Join(sealApiKeyError, sealSecretKeyError); sealError != nil {
		// Restated in this feature's words, since the shared sentinel names the Telegram bot key.
		if errors.Is(sealError, domains.ErrSecretSealUnavailable) {
			return dto.BinanceTradingKeyDto{}, domains.ErrBinanceTradingKeySealUnavailable
		}

		return dto.BinanceTradingKeyDto{}, sealError
	}

	verification, verifyError := binanceTradingKeyService.tradingKeyVerificationProxy.VerifyTradingKey(
		executionContext, credential)
	if verifyError != nil {
		return dto.BinanceTradingKeyDto{}, verifyError
	}

	tradableMarkets, refusal := domains.NewBinanceTradingKeyVerificationDomain(verification).TradableMarkets()
	if refusal != nil {
		return dto.BinanceTradingKeyDto{}, refusal
	}

	savedTradingKey, saveError := binanceTradingKeyService.binanceTradingKeyRepository.Replace(
		executionContext,
		tradingKey.ToEntity(userID, sealedApiKey, sealedSecretKey, tradableMarkets),
		tradableMarkets.UncoveredBotMarketDataKinds())
	if saveError != nil {
		return dto.BinanceTradingKeyDto{}, saveError
	}

	return savedTradingKey.ToDto(), nil
}

// RemoveTradingKey also switches every one of the user's bots' auto order off; removing nothing is not a failure.
func (binanceTradingKeyService *BinanceTradingKeyService) RemoveTradingKey(
	executionContext context.Context, userID uint,
) error {
	return binanceTradingKeyService.binanceTradingKeyRepository.DeleteByUser(executionContext, userID)
}

package domains

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// apiKeyTailLength is a constant, not a setting, so the mask cannot be widened by configuration.
const apiKeyTailLength = 4

// BinanceTradingKeyDomain is a validated pair of trading key strings, both trimmed because pasted blanks are never intended.
type BinanceTradingKeyDomain struct {
	apiKey    string
	secretKey string
}

func NewBinanceTradingKeyDomain(
	binanceTradingKeyWriteDto dto.BinanceTradingKeyWriteDto,
) (BinanceTradingKeyDomain, error) {
	apiKey := strings.TrimSpace(binanceTradingKeyWriteDto.ApiKey)
	if apiKey == "" {
		return BinanceTradingKeyDomain{}, fmt.Errorf(
			"%w: 必須給 API Key", ErrBinanceTradingKeyValidation)
	}

	secretKey := strings.TrimSpace(binanceTradingKeyWriteDto.SecretKey)
	if secretKey == "" {
		return BinanceTradingKeyDomain{}, fmt.Errorf(
			"%w: 必須給 Secret Key", ErrBinanceTradingKeyValidation)
	}

	// A key pasted across lines can never be sent, so it is refused here rather than blamed on Binance.
	breaksInside := func(character rune) bool { return unicode.IsSpace(character) || unicode.IsControl(character) }
	if strings.ContainsFunc(apiKey, breaksInside) {
		return BinanceTradingKeyDomain{}, fmt.Errorf(
			"%w: API Key 中間不能有空白或換行", ErrBinanceTradingKeyValidation)
	}
	if strings.ContainsFunc(secretKey, breaksInside) {
		return BinanceTradingKeyDomain{}, fmt.Errorf(
			"%w: Secret Key 中間不能有空白或換行", ErrBinanceTradingKeyValidation)
	}

	return BinanceTradingKeyDomain{apiKey: apiKey, secretKey: secretKey}, nil
}

func (binanceTradingKeyDomain BinanceTradingKeyDomain) ToCredentialVo() vo.TradingKeyCredentialVo {
	return vo.TradingKeyCredentialVo{
		ApiKey:    binanceTradingKeyDomain.apiKey,
		SecretKey: binanceTradingKeyDomain.secretKey,
	}
}

// ToEntity takes already-sealed strings because sealing is crypto the domain must not know; a key no longer than the tail shows no tail, so the mask never reveals a whole key.
func (binanceTradingKeyDomain BinanceTradingKeyDomain) ToEntity(
	userID uint, sealedApiKey string, sealedSecretKey string, tradableMarkets TradableMarketsDomain,
) entities.BinanceTradingKey {
	apiKeyTail := ""
	if len(binanceTradingKeyDomain.apiKey) > apiKeyTailLength {
		apiKeyTail = binanceTradingKeyDomain.apiKey[len(binanceTradingKeyDomain.apiKey)-apiKeyTailLength:]
	}

	return entities.BinanceTradingKey{
		UserID:                 userID,
		SealedApiKey:           sealedApiKey,
		SealedSecretKey:        sealedSecretKey,
		ApiKeyTail:             apiKeyTail,
		SpotTradingEnabled:     tradableMarkets.Covers(vo.TradableMarketSpot),
		ContractTradingEnabled: tradableMarkets.Covers(vo.TradableMarketContract),
	}
}

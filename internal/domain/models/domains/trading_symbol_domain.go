package domains

import (
	"errors"
	"strings"
)

// nulCharacter is refused in text because PostgreSQL text columns cannot store it, and
// refusing here reports a bad request instead of a storage failure.
const nulCharacter = '\x00'

// TradingSymbolDomain centralises symbol validation and normalisation; callers wrap its
// errors in their own sentinels.
type TradingSymbolDomain struct {
	value string
}

func NewTradingSymbolDomain(symbol string) (TradingSymbolDomain, error) {
	if symbol == "" {
		return TradingSymbolDomain{}, errors.New("必須指定交易標的")
	}

	if strings.ContainsRune(symbol, nulCharacter) {
		return TradingSymbolDomain{}, errors.New("交易標的不得包含空字元（NUL）")
	}

	// Symbols are upper-cased rather than refused because stores are case-sensitive and
	// normalising here keeps reads and writes consistent; digit-only codes are unaffected.
	return TradingSymbolDomain{value: strings.ToUpper(symbol)}, nil
}

func (tradingSymbolDomain TradingSymbolDomain) Value() string {
	return tradingSymbolDomain.value
}

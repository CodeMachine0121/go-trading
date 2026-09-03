package domains

import (
	"errors"
	"strings"
)

// nulCharacter is the one byte PostgreSQL will not hold in a text column whatever
// else it holds. Text carrying it is refused by whichever model owns that text,
// where refusing is an answer about what was asked for — rather than at the
// database, where the same text becomes a storage failure and is reported as though
// the system had broken instead of the request.
const nulCharacter = '\x00'

// TradingSymbolDomain is one trading symbol, checked.
//
// It exists because the same two questions were being asked of a symbol in four
// places and answered in two. Every path asked whether a symbol was there; only the
// paths that write asked whether it could be stored. The ones that did not handed a
// symbol PostgreSQL refuses straight to PostgreSQL, and a request that was merely
// wrong came back as a server that was broken — on five endpoints, which is what
// happens to a rule that lives in as many places as it has callers.
//
// It carries no sentinel of its own. Each caller wraps what comes back in the
// sentinel its own callers already recognise, so reaching this model costs a reader
// no fourth thing to know about.
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

	return TradingSymbolDomain{value: symbol}, nil
}

// Value is the symbol as it is stored and queried.
func (tradingSymbolDomain TradingSymbolDomain) Value() string {
	return tradingSymbolDomain.value
}

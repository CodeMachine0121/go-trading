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

	// Upper case, because a symbol is a name rather than text: btcusdt and BTCUSDT
	// are the same instrument everywhere except in a comparison, and every store
	// this system asks is case-sensitive. Normalising at the one place that both
	// reads and writes pass through is what makes a write and a later read agree —
	// anywhere further out, the two sides can be normalised differently, which is
	// the only way this class of bug survives at all.
	//
	// It is not a refusal, and deliberately so. Lower case is not a mistake anybody
	// can see: it looks exactly like the symbol it means, so a refusal would be the
	// system insisting on a distinction that nothing downstream honours.
	//
	// Symbols that carry no case — a Taiwanese listing's digits, 0050 and 2330 —
	// pass through unchanged, so this costs those venues nothing.
	return TradingSymbolDomain{value: strings.ToUpper(symbol)}, nil
}

// Value is the symbol as it is stored and queried, already normalised — so a caller
// never has to remember to do it, and no caller can do it differently.
func (tradingSymbolDomain TradingSymbolDomain) Value() string {
	return tradingSymbolDomain.value
}

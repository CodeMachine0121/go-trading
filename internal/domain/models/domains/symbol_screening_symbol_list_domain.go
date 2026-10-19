package domains

import (
	"fmt"
	"strings"
)

// SymbolScreeningSymbolListDomain tidies a pasted list: full-width commas count, case and blanks do not, and the first of any repeat wins.
type SymbolScreeningSymbolListDomain struct {
	symbols []string
}

func NewSymbolScreeningSymbolListDomain(
	pasted string, maximumCount int, allowsEmpty bool,
) (SymbolScreeningSymbolListDomain, error) {
	symbols := []string{}
	seen := map[string]bool{}
	for _, piece := range strings.Split(strings.ReplaceAll(pasted, "，", ","), ",") {
		symbol := strings.ToUpper(strings.TrimSpace(piece))
		if symbol == "" || seen[symbol] {
			continue
		}
		for _, character := range symbol {
			if (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
				return SymbolScreeningSymbolListDomain{}, fmt.Errorf(
					"%w: %s 只能有英文字母與數字", ErrSymbolScreeningValidation, symbol)
			}
		}
		seen[symbol] = true
		symbols = append(symbols, symbol)
	}

	if len(symbols) == 0 && !allowsEmpty {
		return SymbolScreeningSymbolListDomain{}, fmt.Errorf("%w: 沒有任何標的", ErrSymbolScreeningValidation)
	}
	if len(symbols) > maximumCount {
		return SymbolScreeningSymbolListDomain{}, fmt.Errorf(
			"%w: 一次最多 %d 個標的", ErrSymbolScreeningValidation, maximumCount)
	}

	return SymbolScreeningSymbolListDomain{symbols: symbols}, nil
}

func (symbolListDomain SymbolScreeningSymbolListDomain) Symbols() []string {
	return symbolListDomain.symbols
}

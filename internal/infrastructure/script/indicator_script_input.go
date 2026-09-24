package script

import (
	"reflect"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptInput is what one kind of market hands a script: the name its entry
// point takes a slice of, and the types a script of that kind may name. It lives apart
// from both sides of a compartment because both need it — the service to say which
// kind it is sending, the compartment to build the matching sandbox — and two copies
// would be two answers the day one of them changed.
type indicatorScriptInput struct {
	// marketKind is how the two sides of a compartment agree on which kind of input
	// is on its way, before any of it has been read.
	marketKind string
	// typeName is how a script names the element it is fed, as in
	// func Calculate(data []indicator.<typeName>). It is only ever read to tell the
	// author what their entry point should look like.
	typeName string
	// types are the types this kind of input lets a script name, keyed by the name it
	// names them by.
	types map[string]reflect.Value
}

// spotIndicatorScriptInput is a spot script's world: it is fed K candles.
var spotIndicatorScriptInput = indicatorScriptInput{
	marketKind: "kCandle",
	typeName:   "KCandle",
	types: map[string]reflect.Value{
		"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
	},
}

// contractIndicatorScriptInput is a contract script's world: it is fed contract bars,
// and may also name the price line a bar carries three of and the spot K candle a bar
// embeds, so that a helper written for indicator.KCandle can be handed the embedded
// candle as it is.
var contractIndicatorScriptInput = indicatorScriptInput{
	marketKind: "contractKCandle",
	typeName:   "ContractKCandle",
	types: map[string]reflect.Value{
		"ContractKCandle": reflect.ValueOf((*vo.ContractKCandleVo)(nil)),
		"PriceLine":       reflect.ValueOf((*vo.PriceLineVo)(nil)),
		"KCandle":         reflect.ValueOf((*vo.KCandleVo)(nil)),
	},
}

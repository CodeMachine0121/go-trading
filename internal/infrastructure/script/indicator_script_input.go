package script

import (
	"reflect"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptInput describes one market kind's script input; shared by the service and the compartment so both agree.
type indicatorScriptInput struct {
	marketKind string
	// typeName is only used to show the author the expected entry-point signature.
	typeName string
	types    map[string]reflect.Value
}

var spotIndicatorScriptInput = indicatorScriptInput{
	marketKind: "kCandle",
	typeName:   "KCandle",
	types: map[string]reflect.Value{
		"KCandle": reflect.ValueOf((*vo.KCandleVo)(nil)),
	},
}

// contractIndicatorScriptInput also exposes the embedded spot KCandle so KCandle helpers can be reused on contract bars.
var contractIndicatorScriptInput = indicatorScriptInput{
	marketKind: "contractKCandle",
	typeName:   "ContractKCandle",
	types: map[string]reflect.Value{
		"ContractKCandle": reflect.ValueOf((*vo.ContractKCandleVo)(nil)),
		"PriceLine":       reflect.ValueOf((*vo.PriceLineVo)(nil)),
		"KCandle":         reflect.ValueOf((*vo.KCandleVo)(nil)),
	},
}

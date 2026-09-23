package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableFillTimings is the entire set a replay may fill by, in the order offered
// back when a declaration is not recognised.
var declarableFillTimings = []vo.FillTimingVo{vo.FillTimingClose, vo.FillTimingNextOpen}

// BacktestFillTimingDomain is at what price a replay fills its signals: the close of
// the bar that spoke, or the open of the next one.
//
// It travels with one replay, like the capital and the costs, and is never written
// back to a strategy script or a trading strategy — the same rules replayed two ways
// is exactly the comparison it exists for.
type BacktestFillTimingDomain struct {
	value vo.FillTimingVo
}

// NewBacktestFillTimingDomain reads what was declared. Declaring nothing is the close,
// which is what every replay was before there was a choice.
func NewBacktestFillTimingDomain(declared string) (BacktestFillTimingDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return BacktestFillTimingDomain{value: vo.FillTimingClose}, nil
	}

	for _, declarableFillTiming := range declarableFillTimings {
		if strings.EqualFold(string(declarableFillTiming), normalizedDeclaration) {
			return BacktestFillTimingDomain{value: declarableFillTiming}, nil
		}
	}

	return BacktestFillTimingDomain{}, fmt.Errorf(
		"成交時點只能是 %s（收盤成交）或 %s（下一格開盤成交）",
		vo.FillTimingClose, vo.FillTimingNextOpen)
}

func (fillTimingDomain BacktestFillTimingDomain) Value() vo.FillTimingVo {
	if fillTimingDomain.value == "" {
		return vo.FillTimingClose
	}

	return fillTimingDomain.value
}

// FillsAtNextOpen is whether a signal waits for the next bar's open.
func (fillTimingDomain BacktestFillTimingDomain) FillsAtNextOpen() bool {
	return fillTimingDomain.value == vo.FillTimingNextOpen
}

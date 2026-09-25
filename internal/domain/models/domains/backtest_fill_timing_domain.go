package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableFillTimings is also the order offered in the error message.
var declarableFillTimings = []vo.FillTimingVo{vo.FillTimingClose, vo.FillTimingNextOpen}

// BacktestFillTimingDomain is whether a replay fills signals at the signalling bar's close or the next bar's open; it is per replay, never saved on a strategy.
type BacktestFillTimingDomain struct {
	value vo.FillTimingVo
}

// NewBacktestFillTimingDomain defaults an empty declaration to the close.
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

func (fillTimingDomain BacktestFillTimingDomain) FillsAtNextOpen() bool {
	return fillTimingDomain.value == vo.FillTimingNextOpen
}

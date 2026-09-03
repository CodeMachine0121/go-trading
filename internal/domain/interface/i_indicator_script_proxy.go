package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_indicator_script_proxy.go -destination=mocks/mock_i_indicator_script_proxy.go -package=mocks

// IIndicatorScriptProxy runs one user-written indicator script over the given K
// candles and hands back one value per indicator name, in the declared kind.
// Everything about how the script is run — the environment, what it may reach, how
// its shape is checked, how failures are caught — lives behind this contract.
type IIndicatorScriptProxy interface {
	Execute(
		executionContext context.Context,
		script string,
		resultType domains.IndicatorResultTypeDomain,
		kCandles []vo.KCandleVo,
	) (map[string]vo.IndicatorValueVo, error)
}

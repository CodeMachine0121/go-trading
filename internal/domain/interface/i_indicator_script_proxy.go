package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_indicator_script_proxy.go -destination=mocks/mock_i_indicator_script_proxy.go -package=mocks

// IIndicatorScriptProxy runs one user-written indicator script over the given K
// candles and hands back one number per indicator name. Everything about how the
// script is run — the environment, what it may reach, how failures are caught —
// lives behind this contract.
type IIndicatorScriptProxy interface {
	Execute(script string, kCandles []vo.KCandleVo) (map[string]float64, error)
}

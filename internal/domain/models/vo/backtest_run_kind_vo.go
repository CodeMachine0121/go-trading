package vo

type BacktestRunKindVo string

const (
	BacktestRunSpotScript              BacktestRunKindVo = "spotScript"
	BacktestRunContractScript          BacktestRunKindVo = "contractScript"
	BacktestRunSpotTradingStrategy     BacktestRunKindVo = "spotTradingStrategy"
	BacktestRunContractTradingStrategy BacktestRunKindVo = "contractTradingStrategy"
)

// NewBacktestRunKindVo reads a stored kind; anything unrecognised reads as empty, which no replay matches.
func NewBacktestRunKindVo(kind string) BacktestRunKindVo {
	switch BacktestRunKindVo(kind) {
	case BacktestRunSpotScript, BacktestRunContractScript,
		BacktestRunSpotTradingStrategy, BacktestRunContractTradingStrategy:
		return BacktestRunKindVo(kind)
	default:
		return ""
	}
}

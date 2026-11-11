package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// declarableStrategyScriptUsages is in the order offered back when a declaration is not recognised.
var declarableStrategyScriptUsages = []vo.StrategyScriptUsageVo{
	vo.StrategyScriptUsageChart,
	vo.StrategyScriptUsageCalculation,
}

// StrategyScriptUsageDomain is where a strategy script may be used; every consumer that computes asks RequireCalculation, so the rule lives here alone.
type StrategyScriptUsageDomain struct {
	value vo.StrategyScriptUsageVo
}

// NewStrategyScriptUsageDomain defaults a blank declaration to calculation, the usage every script had before the choice existed, and matches case-insensitively.
func NewStrategyScriptUsageDomain(declared string) (StrategyScriptUsageDomain, error) {
	normalizedDeclaration := strings.TrimSpace(declared)
	if normalizedDeclaration == "" {
		return StrategyScriptUsageDomain{value: vo.StrategyScriptUsageCalculation}, nil
	}

	for _, declarableUsage := range declarableStrategyScriptUsages {
		if strings.EqualFold(string(declarableUsage), normalizedDeclaration) {
			return StrategyScriptUsageDomain{value: declarableUsage}, nil
		}
	}

	declarableSpellings := make([]string, 0, len(declarableStrategyScriptUsages))
	for _, declarableUsage := range declarableStrategyScriptUsages {
		declarableSpellings = append(declarableSpellings, string(declarableUsage))
	}

	return StrategyScriptUsageDomain{}, fmt.Errorf(
		"%w: 策略腳本的用途只能是 %s 其中之一（圖表用、計算用）",
		ErrStrategyScriptValidation, strings.Join(declarableSpellings, "、"))
}

func (strategyScriptUsageDomain StrategyScriptUsageDomain) Value() vo.StrategyScriptUsageVo {
	return strategyScriptUsageDomain.value
}

func (strategyScriptUsageDomain StrategyScriptUsageDomain) IsChart() bool {
	return strategyScriptUsageDomain.value == vo.StrategyScriptUsageChart
}

// RequireCalculation refuses a chart script for a consumer that computes; consumerPhrase finishes "不能…", e.g. "當信號來源".
func (strategyScriptUsageDomain StrategyScriptUsageDomain) RequireCalculation(consumerPhrase string) error {
	if strategyScriptUsageDomain.IsChart() {
		return fmt.Errorf("%w: 這支策略腳本是圖表用，不能%s——要用請先把它切換成計算用",
			ErrStrategyScriptUsageNotCalculation, consumerPhrase)
	}

	return nil
}

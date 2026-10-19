package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SymbolScreeningBaselineDomain is the administrator's current portfolio that passed candidates are compared with; one symbol that cannot be replayed leaves the whole comparison out.
type SymbolScreeningBaselineDomain struct {
	period            SymbolScreeningPeriodDomain
	closedTrades      []dto.ContractClosedTradeDto
	unavailableReason string
}

func NewSymbolScreeningBaselineDomain(period SymbolScreeningPeriodDomain) SymbolScreeningBaselineDomain {
	return SymbolScreeningBaselineDomain{period: period, closedTrades: []dto.ContractClosedTradeDto{}}
}

func (baselineDomain SymbolScreeningBaselineDomain) Including(
	segmentResults []dto.ContractBacktestResultDto,
) SymbolScreeningBaselineDomain {
	baselineDomain.closedTrades = append(
		append([]dto.ContractClosedTradeDto{}, baselineDomain.closedTrades...), baselineDomain.period.ClosedTradesOf(segmentResults)...)

	return baselineDomain
}

func (baselineDomain SymbolScreeningBaselineDomain) Unavailable(symbol string, cause error) SymbolScreeningBaselineDomain {
	baselineDomain.unavailableReason = fmt.Sprintf("現有組合標的 %s 無法比較：%s", symbol, cause.Error())

	return baselineDomain
}

// UnavailableReason is empty unless a baseline symbol could not be replayed.
func (baselineDomain SymbolScreeningBaselineDomain) UnavailableReason() string {
	return baselineDomain.unavailableReason
}

// Compared adds each passed candidate's contribution to the baseline, leaving the candidates as they were when the baseline is unavailable.
func (baselineDomain SymbolScreeningBaselineDomain) Compared(
	candidates []entities.SymbolScreeningCandidate, closedTradesBySymbol map[string][]dto.ContractClosedTradeDto,
) []entities.SymbolScreeningCandidate {
	if baselineDomain.unavailableReason != "" {
		return candidates
	}

	period := baselineDomain.period
	baseline := NewSymbolScreeningHoldingTimelineDomain(baselineDomain.closedTrades, period.StartTime(), period.EndTime())
	compared := append([]entities.SymbolScreeningCandidate{}, candidates...)
	for index, candidate := range compared {
		if candidate.Status != string(vo.SymbolScreeningCandidatePassed) {
			continue
		}
		contribution := NewSymbolScreeningHoldingTimelineDomain(
			closedTradesBySymbol[candidate.Symbol], period.StartTime(), period.EndTime()).ContributionAgainst(baseline)
		compared[index].Findings.Contribution = &contribution
	}

	return compared
}

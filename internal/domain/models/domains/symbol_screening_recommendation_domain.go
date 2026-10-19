package domains

import (
	"cmp"
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SymbolScreeningRecommendationDomain puts passed candidates first, those that shorten the baseline's longest gap most ahead, then by return without the best segment; the rest keep the pasted order.
type SymbolScreeningRecommendationDomain struct {
	candidates []entities.SymbolScreeningCandidate
}

func NewSymbolScreeningRecommendationDomain(
	candidates []entities.SymbolScreeningCandidate,
) SymbolScreeningRecommendationDomain {
	return SymbolScreeningRecommendationDomain{candidates: slices.Clone(candidates)}
}

func (recommendationDomain SymbolScreeningRecommendationDomain) Ranked() []entities.SymbolScreeningCandidate {
	ranked := slices.Clone(recommendationDomain.candidates)
	slices.SortStableFunc(ranked, func(earlier, later entities.SymbolScreeningCandidate) int {
		earlierPassed := earlier.Status == string(vo.SymbolScreeningCandidatePassed)
		laterPassed := later.Status == string(vo.SymbolScreeningCandidatePassed)
		if earlierPassed != laterPassed {
			if earlierPassed {
				return -1
			}
			return 1
		}
		if earlierPassed {
			if earlier.Findings.Contribution != nil && later.Findings.Contribution != nil {
				if byGap := cmp.Compare(later.Findings.Contribution.GapReductionDays,
					earlier.Findings.Contribution.GapReductionDays); byGap != 0 {
					return byGap
				}
			}
			earlierReturn, laterReturn := 0.0, 0.0
			if earlier.Findings.ReturnWithoutBestSegment != nil {
				earlierReturn = *earlier.Findings.ReturnWithoutBestSegment
			}
			if later.Findings.ReturnWithoutBestSegment != nil {
				laterReturn = *later.Findings.ReturnWithoutBestSegment
			}
			if byReturn := cmp.Compare(laterReturn, earlierReturn); byReturn != 0 {
				return byReturn
			}
		}

		return cmp.Compare(earlier.Position, later.Position)
	})
	for index := range ranked {
		rank := index + 1
		ranked[index].RecommendationRank = &rank
	}

	return ranked
}

// InDisplayOrder shows ranked candidates by rank and unranked ones, during a run, by the pasted order.
func (recommendationDomain SymbolScreeningRecommendationDomain) InDisplayOrder() []entities.SymbolScreeningCandidate {
	ordered := slices.Clone(recommendationDomain.candidates)
	slices.SortStableFunc(ordered, func(earlier, later entities.SymbolScreeningCandidate) int {
		if earlier.RecommendationRank != nil && later.RecommendationRank != nil {
			return cmp.Compare(*earlier.RecommendationRank, *later.RecommendationRank)
		}

		return cmp.Compare(earlier.Position, later.Position)
	})

	return ordered
}

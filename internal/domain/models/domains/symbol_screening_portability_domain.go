package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SymbolScreeningPortabilityDomain judges the unchanged strategy on every segment, in order: enough trades, most segments winning, still winning without the best one.
type SymbolScreeningPortabilityDomain struct {
	segments          []vo.SymbolScreeningSegmentVo
	results           []dto.ContractBacktestResultDto
	minimumTradeCount int
}

func NewSymbolScreeningPortabilityDomain(
	segments []vo.SymbolScreeningSegmentVo, results []dto.ContractBacktestResultDto, minimumTradeCount int,
) SymbolScreeningPortabilityDomain {
	return SymbolScreeningPortabilityDomain{segments: segments, results: results, minimumTradeCount: minimumTradeCount}
}

// Judge adds the segment figures to the findings whatever the verdict, so a rejection can be checked by eye.
func (portabilityDomain SymbolScreeningPortabilityDomain) Judge(
	findings dto.SymbolScreeningFindingsDto,
) (bool, string, dto.SymbolScreeningFindingsDto) {
	closedTradeCount := 0
	winningSegmentCount := 0
	bestIndex := 0
	segmentFindings := make([]dto.SymbolScreeningSegmentFindingDto, 0, len(portabilityDomain.results))
	for index, result := range portabilityDomain.results {
		segment := portabilityDomain.segments[index]
		returnRate := result.Summary.TotalReturnRate
		segmentFindings = append(segmentFindings, dto.SymbolScreeningSegmentFindingDto{
			StartTime:        segment.StartTime,
			EndTime:          segment.EndTime,
			ReturnRate:       returnRate,
			MaximumDrawdown:  result.Summary.MaximumDrawdown,
			ClosedTradeCount: len(result.ClosedTrades),
			LiquidationCount: result.Summary.LiquidationExitCount,
		})
		closedTradeCount += len(result.ClosedTrades)
		if returnRate > 0 {
			winningSegmentCount++
		}
		if returnRate > portabilityDomain.results[bestIndex].Summary.TotalReturnRate {
			bestIndex = index
		}
	}

	compounded := 1.0
	for index, result := range portabilityDomain.results {
		if index != bestIndex {
			compounded *= 1 + result.Summary.TotalReturnRate
		}
	}
	returnWithoutBestSegment := compounded - 1
	findings.Segments = segmentFindings
	findings.ReturnWithoutBestSegment = &returnWithoutBestSegment

	switch {
	case closedTradeCount < portabilityDomain.minimumTradeCount:
		return false, fmt.Sprintf("交易太少，無法判斷（共 %d 筆，至少要 %d 筆）",
			closedTradeCount, portabilityDomain.minimumTradeCount), findings
	case winningSegmentCount*2 <= len(portabilityDomain.results):
		return false, fmt.Sprintf("多數區段虧損（%d 段裡只有 %d 段賺錢）",
			len(portabilityDomain.results), winningSegmentCount), findings
	case returnWithoutBestSegment <= 0:
		return false, fmt.Sprintf("去掉最好的一段就虧損（%.2f%%）", returnWithoutBestSegment*100), findings
	}

	return true, "", findings
}

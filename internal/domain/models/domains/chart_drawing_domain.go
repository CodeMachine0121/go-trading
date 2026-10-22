package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingDomain is one drawing that can be drawn: a known kind, the right number of anchors with positive prices, and a valid look.
type ChartDrawingDomain struct {
	kind       ChartDrawingKindDomain
	anchors    []dto.ChartDrawingAnchorDto
	style      ChartDrawingStyleDomain
	provenance vo.ChartDrawingProvenanceVo
}

// NewChartDrawingDomain checks the drawing alone; anchors are never compared with candles here, since a person draws wherever they mean to.
func NewChartDrawingDomain(
	kindValue string, anchorDtos []dto.ChartDrawingAnchorDto, styleDto dto.ChartDrawingStyleWriteDto,
	provenance vo.ChartDrawingProvenanceVo,
) (ChartDrawingDomain, error) {
	kind, kindError := NewChartDrawingKindDomain(kindValue)
	if kindError != nil {
		return ChartDrawingDomain{}, kindError
	}

	if anchorCountError := kind.RequireAnchorCount(len(anchorDtos)); anchorCountError != nil {
		return ChartDrawingDomain{}, anchorCountError
	}

	anchors := make([]dto.ChartDrawingAnchorDto, 0, len(anchorDtos))
	for _, anchorDto := range anchorDtos {
		if anchorDto.OpenTime.IsZero() {
			return ChartDrawingDomain{}, errors.New("每個錨點都要有起始時間")
		}
		if !anchorDto.Price.IsPositive() {
			return ChartDrawingDomain{}, errors.New("錨點價格必須大於零")
		}

		anchors = append(anchors, dto.ChartDrawingAnchorDto{OpenTime: anchorDto.OpenTime.UTC(), Price: anchorDto.Price})
	}

	style, styleError := NewChartDrawingStyleDomain(kind, styleDto)
	if styleError != nil {
		return ChartDrawingDomain{}, styleError
	}

	return ChartDrawingDomain{kind: kind, anchors: anchors, style: style, provenance: provenance}, nil
}

func (chartDrawingDomain ChartDrawingDomain) AnchorCount() int {
	return len(chartDrawingDomain.anchors)
}

func (chartDrawingDomain ChartDrawingDomain) ToDto(id uint) dto.ChartDrawingDto {
	return dto.ChartDrawingDto{
		ID:                      id,
		Kind:                    string(chartDrawingDomain.kind.Value()),
		Anchors:                 chartDrawingDomain.anchors,
		Color:                   string(chartDrawingDomain.style.color),
		LineWidth:               chartDrawingDomain.style.lineWidth,
		LineStyle:               string(chartDrawingDomain.style.lineStyle),
		Locked:                  chartDrawingDomain.style.locked,
		Visible:                 chartDrawingDomain.style.visible,
		Source:                  string(chartDrawingDomain.provenance.Source),
		AssistantConversationID: chartDrawingDomain.provenance.AssistantConversationID,
		AssistantTurnID:         chartDrawingDomain.provenance.AssistantTurnID,
	}
}

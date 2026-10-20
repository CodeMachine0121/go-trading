package domains

import (
	"errors"
	"fmt"
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	chartDrawingMinimumLineWidth = 1
	chartDrawingMaximumLineWidth = 4
)

var chartDrawingColors = []vo.ChartDrawingColorVo{
	vo.ChartDrawingColorRed, vo.ChartDrawingColorOrange, vo.ChartDrawingColorYellow, vo.ChartDrawingColorGreen,
	vo.ChartDrawingColorTeal, vo.ChartDrawingColorCyan, vo.ChartDrawingColorBlue, vo.ChartDrawingColorIndigo,
	vo.ChartDrawingColorPurple, vo.ChartDrawingColorPink, vo.ChartDrawingColorGray, vo.ChartDrawingColorSilver,
}

var chartDrawingLineStyles = []vo.ChartDrawingLineStyleVo{
	vo.ChartDrawingLineStyleSolid, vo.ChartDrawingLineStyleDashed, vo.ChartDrawingLineStyleDotted,
}

// ChartDrawingStyleDomain is how a drawing looks; anything not given takes its kind's default, unlocked and shown.
type ChartDrawingStyleDomain struct {
	color     vo.ChartDrawingColorVo
	lineWidth int
	lineStyle vo.ChartDrawingLineStyleVo
	locked    bool
	visible   bool
}

func NewChartDrawingStyleDomain(
	kind ChartDrawingKindDomain, styleDto dto.ChartDrawingStyleWriteDto,
) (ChartDrawingStyleDomain, error) {
	style := ChartDrawingStyleDomain{
		color:     kind.DefaultColor(),
		lineWidth: kind.DefaultLineWidth(),
		lineStyle: vo.ChartDrawingLineStyleSolid,
		locked:    false,
		visible:   true,
	}

	if styleDto.Color != "" {
		style.color = vo.ChartDrawingColorVo(styleDto.Color)
		if !slices.Contains(chartDrawingColors, style.color) {
			return ChartDrawingStyleDomain{}, fmt.Errorf(
				"線色只能是調色盤上的十二色之一（red、orange、yellow、green、teal、cyan、blue、indigo、purple、pink、gray、silver），收到的是「%s」",
				styleDto.Color)
		}
	}

	if styleDto.LineWidth != 0 {
		style.lineWidth = styleDto.LineWidth
		if style.lineWidth < chartDrawingMinimumLineWidth || style.lineWidth > chartDrawingMaximumLineWidth {
			return ChartDrawingStyleDomain{}, fmt.Errorf("線寬只能是 %d 到 %d，收到的是 %d",
				chartDrawingMinimumLineWidth, chartDrawingMaximumLineWidth, styleDto.LineWidth)
		}
	}

	if styleDto.LineStyle != "" {
		style.lineStyle = vo.ChartDrawingLineStyleVo(styleDto.LineStyle)
		if !slices.Contains(chartDrawingLineStyles, style.lineStyle) {
			return ChartDrawingStyleDomain{}, errors.New("線型只能是實線（solid）、虛線（dashed）、點線（dotted）")
		}
	}

	if styleDto.Locked != nil {
		style.locked = *styleDto.Locked
	}
	if styleDto.Visible != nil {
		style.visible = *styleDto.Visible
	}

	return style, nil
}

package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// chartDrawingKindEntry keeps a kind's name, anchor count and default look together; adding a kind is adding one entry.
type chartDrawingKindEntry struct {
	value            vo.ChartDrawingKindVo
	label            string
	anchorCount      int
	defaultColor     vo.ChartDrawingColorVo
	defaultLineWidth int
}

// chartDrawingKinds matches the chart workspace's catalogue, including its default styles.
var chartDrawingKinds = []chartDrawingKindEntry{
	{value: vo.ChartDrawingKindTrendLine, label: "趨勢線", anchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 2},
	{value: vo.ChartDrawingKindRay, label: "射線", anchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 2},
	{value: vo.ChartDrawingKindHorizontalLine, label: "水平線", anchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindHorizontalRay, label: "水平射線", anchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindVerticalLine, label: "垂直線", anchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindFibonacciRetracement, label: "斐波那契回撤", anchorCount: 2, defaultColor: vo.ChartDrawingColorGray, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindRectangle, label: "矩形", anchorCount: 2, defaultColor: vo.ChartDrawingColorPurple, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindDateAndPriceRange, label: "日期與價格範圍", anchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
}

type ChartDrawingKindDomain struct {
	entry chartDrawingKindEntry
}

func NewChartDrawingKindDomain(declared string) (ChartDrawingKindDomain, error) {
	for _, entry := range chartDrawingKinds {
		if string(entry.value) == declared {
			return ChartDrawingKindDomain{entry: entry}, nil
		}
	}

	spellings := make([]string, 0, len(chartDrawingKinds))
	for _, entry := range chartDrawingKinds {
		spellings = append(spellings, string(entry.value))
	}

	return ChartDrawingKindDomain{}, fmt.Errorf("不認得的繪圖種類「%s」，只能是 %s 其中之一",
		declared, strings.Join(spellings, "、"))
}

func (chartDrawingKindDomain ChartDrawingKindDomain) Value() vo.ChartDrawingKindVo {
	return chartDrawingKindDomain.entry.value
}

// RequireAnchorCount names the kind in its own words so the refusal reads like the chart's toolbar.
func (chartDrawingKindDomain ChartDrawingKindDomain) RequireAnchorCount(givenCount int) error {
	if givenCount != chartDrawingKindDomain.entry.anchorCount {
		return fmt.Errorf("%s要 %d 個錨點，收到 %d 個",
			chartDrawingKindDomain.entry.label, chartDrawingKindDomain.entry.anchorCount, givenCount)
	}

	return nil
}

func (chartDrawingKindDomain ChartDrawingKindDomain) DefaultColor() vo.ChartDrawingColorVo {
	return chartDrawingKindDomain.entry.defaultColor
}

func (chartDrawingKindDomain ChartDrawingKindDomain) DefaultLineWidth() int {
	return chartDrawingKindDomain.entry.defaultLineWidth
}

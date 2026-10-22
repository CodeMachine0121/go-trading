package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// chartDrawingKindEntry keeps a kind's name, anchor counts, default look and who may draw it together; adding a kind is adding one entry.
type chartDrawingKindEntry struct {
	value              vo.ChartDrawingKindVo
	label              string
	minimumAnchorCount int
	maximumAnchorCount int
	drawnByHandOnly    bool
	defaultColor       vo.ChartDrawingColorVo
	defaultLineWidth   int
}

// chartDrawingKinds matches the chart workspace's catalogue, including its default styles.
var chartDrawingKinds = []chartDrawingKindEntry{
	{value: vo.ChartDrawingKindTrendLine, label: "趨勢線", minimumAnchorCount: 2, maximumAnchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 2},
	{value: vo.ChartDrawingKindRay, label: "射線", minimumAnchorCount: 2, maximumAnchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 2},
	{value: vo.ChartDrawingKindHorizontalLine, label: "水平線", minimumAnchorCount: 1, maximumAnchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindHorizontalRay, label: "水平射線", minimumAnchorCount: 1, maximumAnchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindVerticalLine, label: "垂直線", minimumAnchorCount: 1, maximumAnchorCount: 1, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindFibonacciRetracement, label: "斐波那契回撤", minimumAnchorCount: 2, maximumAnchorCount: 2, defaultColor: vo.ChartDrawingColorGray, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindRectangle, label: "矩形", minimumAnchorCount: 2, maximumAnchorCount: 2, defaultColor: vo.ChartDrawingColorPurple, defaultLineWidth: 1},
	{value: vo.ChartDrawingKindDateAndPriceRange, label: "日期與價格範圍", minimumAnchorCount: 2, maximumAnchorCount: 2, defaultColor: vo.ChartDrawingColorBlue, defaultLineWidth: 1},
	// A brush stroke follows the hand, so its anchors are as many points as the stroke kept and the assistant, which pins to candles, cannot draw one.
	{value: vo.ChartDrawingKindBrush, label: "奇異筆", minimumAnchorCount: 2, maximumAnchorCount: 200, drawnByHandOnly: true, defaultColor: vo.ChartDrawingColorOrange, defaultLineWidth: 2},
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
	entry := chartDrawingKindDomain.entry
	if givenCount >= entry.minimumAnchorCount && givenCount <= entry.maximumAnchorCount {
		return nil
	}

	if entry.minimumAnchorCount == entry.maximumAnchorCount {
		return fmt.Errorf("%s要 %d 個錨點，收到 %d 個", entry.label, entry.minimumAnchorCount, givenCount)
	}

	return fmt.Errorf("%s要 %d 到 %d 個錨點，收到 %d 個",
		entry.label, entry.minimumAnchorCount, entry.maximumAnchorCount, givenCount)
}

func (chartDrawingKindDomain ChartDrawingKindDomain) RequireAssistantDrawable() error {
	if chartDrawingKindDomain.entry.drawnByHandOnly {
		return fmt.Errorf("%s只能由使用者手畫，助手不能畫", chartDrawingKindDomain.entry.label)
	}

	return nil
}

func (chartDrawingKindDomain ChartDrawingKindDomain) DefaultColor() vo.ChartDrawingColorVo {
	return chartDrawingKindDomain.entry.defaultColor
}

func (chartDrawingKindDomain ChartDrawingKindDomain) DefaultLineWidth() int {
	return chartDrawingKindDomain.entry.defaultLineWidth
}

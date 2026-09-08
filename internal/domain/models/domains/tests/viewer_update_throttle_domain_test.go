package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

// The ceiling exists so a market trading many times a second does not make the
// screen busy without making it clearer. What it must never swallow is a candle's
// last word.
func TestAdmitLetsAFormingCandleThroughOncePerCeiling(t *testing.T) {
	testCases := []struct {
		name          string
		secondsPassed []int
		closed        []bool
		admitted      []bool
	}{
		{
			name:          "十秒內成交上百筆只送一次",
			secondsPassed: []int{10, 10, 11, 12, 15},
			closed:        []bool{false, false, false, false, false},
			admitted:      []bool{true, false, false, false, false},
		},
		{
			name:          "距上次送出僅兩秒的變動先不送，滿十秒才送",
			secondsPassed: []int{10, 12, 20},
			closed:        []bool{false, false, false},
			admitted:      []bool{true, false, true},
		},
		{
			name:          "距上次送出僅兩秒，那一根走完就立刻送",
			secondsPassed: []int{10, 12},
			closed:        []bool{false, true},
			admitted:      []bool{true, true},
		},
		{
			name:          "一根走完永遠送得出去，連續幾根都一樣",
			secondsPassed: []int{1, 2, 3},
			closed:        []bool{true, true, true},
			admitted:      []bool{true, true, true},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			throttleDomain := domains.NewViewerUpdateThrottleDomain(
				10*time.Second, followStartedAt)

			for index, secondsPassed := range testCase.secondsPassed {
				liveKCandle := formingKCandle()
				if testCase.closed[index] {
					liveKCandle = closedKCandle()
				}

				admitted := throttleDomain.Admit(
					liveKCandle, followStartedAt.Add(time.Duration(secondsPassed)*time.Second))

				assert.Equal(t, testCase.admitted[index], admitted,
					"第 %d 次（過了 %d 秒，走完=%v）", index+1, secondsPassed, testCase.closed[index])
			}
		})
	}
}

// Two symbols travelling on one channel are two pictures. Holding one back because
// the other just moved would make a busy neighbour into a slow chart.
func TestEachSymbolIsThrottledOnItsOwn(t *testing.T) {
	firstThrottle := domains.NewViewerUpdateThrottleDomain(10*time.Second, followStartedAt)
	secondThrottle := domains.NewViewerUpdateThrottleDomain(10*time.Second, followStartedAt)

	assert.True(t, firstThrottle.Admit(formingKCandle(), followStartedAt.Add(10*time.Second)))
	assert.False(t, firstThrottle.Admit(formingKCandle(), followStartedAt.Add(11*time.Second)))

	assert.True(t, secondThrottle.Admit(formingKCandle(), followStartedAt.Add(11*time.Second)),
		"另一檔剛剛送過，不該讓這一檔的第一次被擋下")
}

// A setting left unfilled means "use the stated rule", never "no rule at all".
func TestAnUnusableCeilingFallsBackToTheStatedRule(t *testing.T) {
	throttleDomain := domains.NewViewerUpdateThrottleDomain(0, followStartedAt)

	assert.False(t, throttleDomain.Admit(formingKCandle(), followStartedAt.Add(9*time.Second)),
		"更新間隔上限應回到十秒")
	assert.True(t, throttleDomain.Admit(formingKCandle(), followStartedAt.Add(10*time.Second)))
}

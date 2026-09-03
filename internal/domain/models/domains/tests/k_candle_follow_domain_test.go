package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

var followStartedAt = time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)

func formingKCandle() vo.LiveKCandleVo {
	return vo.LiveKCandleVo{Symbol: "BTCUSDT", OpenTime: followStartedAt, Closed: false}
}

func closedKCandle() vo.LiveKCandleVo {
	return vo.LiveKCandleVo{Symbol: "BTCUSDT", OpenTime: followStartedAt, Closed: true}
}

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
			followDomain := domains.NewKCandleFollowDomain(
				10*time.Second, 30*time.Second, 30*time.Second, followStartedAt)

			for index, secondsPassed := range testCase.secondsPassed {
				liveKCandle := formingKCandle()
				if testCase.closed[index] {
					liveKCandle = closedKCandle()
				}

				admitted := followDomain.Admit(
					liveKCandle, followStartedAt.Add(time.Duration(secondsPassed)*time.Second))

				assert.Equal(t, testCase.admitted[index], admitted,
					"第 %d 次（過了 %d 秒，走完=%v）", index+1, secondsPassed, testCase.closed[index])
			}
		})
	}
}

// A connection that looks open but has stopped delivering is how this kind of feed
// usually dies. Silence is therefore read as death — a needless reconnection costs
// far less than a viewer trusting a frozen picture.
func TestHasGoneQuietOnceNothingHasArrivedForTheThreshold(t *testing.T) {
	testCases := []struct {
		name           string
		receivedAfter  []int
		askedAt        int
		expectedIsDead bool
	}{
		{name: "剛開始跟就問，還不算死", receivedAfter: []int{}, askedAt: 0, expectedIsDead: false},
		{name: "差一秒到門檻，還不算死", receivedAfter: []int{}, askedAt: 29, expectedIsDead: false},
		{name: "剛好滿門檻，算死", receivedAfter: []int{}, askedAt: 30, expectedIsDead: true},
		{name: "遠超過門檻，算死", receivedAfter: []int{}, askedAt: 120, expectedIsDead: true},
		{name: "中途收到東西，門檻從那時重算", receivedAfter: []int{25}, askedAt: 50, expectedIsDead: false},
		{name: "中途收到東西之後又安靜夠久，算死", receivedAfter: []int{25}, askedAt: 55, expectedIsDead: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			followDomain := domains.NewKCandleFollowDomain(
				10*time.Second, 30*time.Second, 30*time.Second, followStartedAt)
			for _, receivedAfter := range testCase.receivedAfter {
				followDomain.Admit(
					formingKCandle(), followStartedAt.Add(time.Duration(receivedAfter)*time.Second))
			}

			isDead := followDomain.HasGoneQuiet(
				followStartedAt.Add(time.Duration(testCase.askedAt) * time.Second))

			assert.Equal(t, testCase.expectedIsDead, isDead)
		})
	}
}

// The gap grows so a source that is briefly unwell is not hammered, and stops
// growing so a source that recovers in an hour is not ignored for another one.
func TestNextRetryDelayGrowsUpToTheCeilingAndNeverGivesUp(t *testing.T) {
	followDomain := domains.NewKCandleFollowDomain(
		10*time.Second, 30*time.Second, 30*time.Second, followStartedAt)

	delays := make([]time.Duration, 0, 8)
	for range 8 {
		delays = append(delays, followDomain.NextRetryDelay())
	}

	assert.Equal(t, []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}, delays)
}

// The ceiling binds every gap, the first one included: asking for gaps no longer
// than half a second never meant "except the first one".
func TestTheFirstRetryGapIsBoundByTheCeilingToo(t *testing.T) {
	followDomain := domains.NewKCandleFollowDomain(
		10*time.Second, 30*time.Second, 200*time.Millisecond, followStartedAt)

	assert.Equal(t, 200*time.Millisecond, followDomain.NextRetryDelay())
	assert.Equal(t, 200*time.Millisecond, followDomain.NextRetryDelay())
}

// Silence is measured at half the threshold so a dead feed is noticed within one
// and a half thresholds rather than two — and the threshold it halves is the
// settled one, not whatever the caller happened to hand over.
func TestSilenceIsCheckedTwicePerThreshold(t *testing.T) {
	assert.Equal(t, 15*time.Second, domains.NewKCandleFollowDomain(
		10*time.Second, 30*time.Second, 30*time.Second, followStartedAt).QuietCheckInterval())
	assert.Equal(t, 15*time.Second, domains.NewKCandleFollowDomain(
		10*time.Second, 0, 30*time.Second, followStartedAt).QuietCheckInterval(),
		"門檻沒設定時，檢查間隔也該跟著回到預設門檻的一半")
}

// Recovering must undo the gap the outage earned, or a follow that comes back would
// keep waiting half a minute between rounds it no longer needs to retry.
func TestFollowingAgainPutsTheRetryGapBackToItsShortest(t *testing.T) {
	followDomain := domains.NewKCandleFollowDomain(
		10*time.Second, 30*time.Second, 30*time.Second, followStartedAt)
	for range 5 {
		followDomain.NextRetryDelay()
	}

	followDomain.MarkFollowing(followStartedAt.Add(time.Minute))

	assert.Equal(t, time.Second, followDomain.NextRetryDelay())
	assert.False(t, followDomain.HasGoneQuiet(followStartedAt.Add(time.Minute+29*time.Second)),
		"重新跟上之後，安靜門檻應從那一刻重新起算")
}

// A setting left unfilled means "use the stated rule", never "no rule at all".
func TestUnusableSettingsFallBackToTheStatedRules(t *testing.T) {
	followDomain := domains.NewKCandleFollowDomain(0, -time.Second, 0, followStartedAt)

	assert.False(t, followDomain.Admit(formingKCandle(), followStartedAt.Add(9*time.Second)),
		"更新間隔上限應回到十秒")
	assert.True(t, followDomain.Admit(formingKCandle(), followStartedAt.Add(10*time.Second)))
	assert.False(t, followDomain.HasGoneQuiet(followStartedAt.Add(39*time.Second)),
		"安靜門檻應回到三十秒（自最後一次收到起算）")
	assert.True(t, followDomain.HasGoneQuiet(followStartedAt.Add(40*time.Second)))
}

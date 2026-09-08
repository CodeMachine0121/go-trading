package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var followStartedAt = time.Date(2026, 9, 3, 9, 0, 0, 0, time.UTC)

func formingKCandle() vo.LiveKCandleVo {
	return vo.LiveKCandleVo{Symbol: "BTCUSDT", OpenTime: followStartedAt, Closed: false}
}

func closedKCandle() vo.LiveKCandleVo {
	return vo.LiveKCandleVo{Symbol: "BTCUSDT", OpenTime: followStartedAt, Closed: true}
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
			healthDomain := domains.NewLiveChannelHealthDomain(
				30*time.Second, 30*time.Second, followStartedAt)
			for _, receivedAfter := range testCase.receivedAfter {
				healthDomain.MarkReceived(
					followStartedAt.Add(time.Duration(receivedAfter) * time.Second))
			}

			isDead := healthDomain.HasGoneQuiet(
				followStartedAt.Add(time.Duration(testCase.askedAt) * time.Second))

			assert.Equal(t, testCase.expectedIsDead, isDead)
		})
	}
}

// The gap grows so a source that is briefly unwell is not hammered, and stops
// growing so a source that recovers in an hour is not ignored for another one.
func TestNextRetryDelayGrowsUpToTheCeilingAndNeverGivesUp(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(
		30*time.Second, 30*time.Second, followStartedAt)

	delays := make([]time.Duration, 0, 8)
	for range 8 {
		delays = append(delays, healthDomain.NextRetryDelay())
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
	healthDomain := domains.NewLiveChannelHealthDomain(
		30*time.Second, 200*time.Millisecond, followStartedAt)

	assert.Equal(t, 200*time.Millisecond, healthDomain.NextRetryDelay())
	assert.Equal(t, 200*time.Millisecond, healthDomain.NextRetryDelay())
}

// Silence is measured at half the threshold so a dead feed is noticed within one
// and a half thresholds rather than two — and the threshold it halves is the
// settled one, not whatever the caller happened to hand over.
func TestSilenceIsCheckedTwicePerThreshold(t *testing.T) {
	assert.Equal(t, 15*time.Second, domains.NewLiveChannelHealthDomain(
		30*time.Second, 30*time.Second, followStartedAt).QuietCheckInterval())
	assert.Equal(t, 15*time.Second, domains.NewLiveChannelHealthDomain(
		0, 30*time.Second, followStartedAt).QuietCheckInterval(),
		"門檻沒設定時，檢查間隔也該跟著回到預設門檻的一半")
}

// Halving the smallest threshold a caller can express rounds down to nothing, and a
// repeating check asked to repeat every nothing is a crash rather than a fast check.
func TestSilenceIsNeverCheckedAtNoIntervalAtAll(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(
		time.Nanosecond, 30*time.Second, followStartedAt)

	assert.Positive(t, healthDomain.QuietCheckInterval())
}

// Recovering must undo the gap the outage earned, or a channel that comes back would
// keep waiting half a minute between rounds it no longer needs to retry. Recovering
// means data arriving — that is the only thing a source can do that proves it works.
func TestReceivingSomethingAgainPutsTheRetryGapBackToItsShortest(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(
		30*time.Second, 30*time.Second, followStartedAt)
	for range 5 {
		healthDomain.NextRetryDelay()
	}

	healthDomain.MarkReceived(followStartedAt.Add(time.Minute))

	assert.Equal(t, time.Second, healthDomain.NextRetryDelay())
	assert.False(t, healthDomain.HasGoneQuiet(followStartedAt.Add(time.Minute+29*time.Second)),
		"重新收到資料之後，安靜門檻應從那一刻重新起算")
}

// The failure this rule exists for: a source that accepts every connection and then
// says nothing. Opening a connection proves nothing, so it must not shorten the gap
// — otherwise every attempt resets it and the source is hammered once a second for
// as long as it stays broken.
func TestConnectingWithoutDeliveringNeverShortensTheRetryGap(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(
		30*time.Second, 30*time.Second, followStartedAt)

	delays := make([]time.Duration, 0, 6)
	for attempt := range 6 {
		// Each round is one connection that opened and delivered nothing before dying.
		healthDomain.MarkConnected(followStartedAt.Add(time.Duration(attempt) * time.Minute))
		delays = append(delays, healthDomain.NextRetryDelay())
	}

	assert.Equal(t, []time.Duration{
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
	}, delays, "連得上但沒資料，間隔仍必須逐次拉長到上限")
}

// A connection that has only just opened has not been silent for however long the
// previous one was, so the silence has to start being measured from here.
func TestAFreshConnectionIsNotInstantlyQuiet(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(
		30*time.Second, 30*time.Second, followStartedAt)
	require.True(t, healthDomain.HasGoneQuiet(followStartedAt.Add(31*time.Second)))

	healthDomain.MarkConnected(followStartedAt.Add(31 * time.Second))

	assert.False(t, healthDomain.HasGoneQuiet(followStartedAt.Add(31*time.Second+29*time.Second)))
	assert.True(t, healthDomain.HasGoneQuiet(followStartedAt.Add(31*time.Second+30*time.Second)))
}

// A setting left unfilled means "use the stated rule", never "no rule at all".
func TestUnusableChannelSettingsFallBackToTheStatedRules(t *testing.T) {
	healthDomain := domains.NewLiveChannelHealthDomain(-time.Second, 0, followStartedAt)

	assert.False(t, healthDomain.HasGoneQuiet(followStartedAt.Add(29*time.Second)),
		"安靜門檻應回到三十秒")
	assert.True(t, healthDomain.HasGoneQuiet(followStartedAt.Add(30*time.Second)))
	assert.Equal(t, time.Second, healthDomain.NextRetryDelay(),
		"重試上限沒設定時，第一段間隔仍是最短的那一個")
}

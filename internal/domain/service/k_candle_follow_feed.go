package service

import (
	"context"
	"log"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleFollowFeed keeps one live line followed against a source, reconnecting with growing back-off; it is market-agnostic, so spot and contract follows share it and decide via report what a candle amounts to.
type kCandleFollowFeed struct {
	liveMarketDataProxy _interface.ILiveMarketDataProxy
	clockProxy          _interface.IClockProxy
	quietTimeout        time.Duration
	maximumRetryDelay   time.Duration
	report              func(context.Context, *kCandleFollowSymbol, vo.LiveKCandleVo)
}

func newKCandleFollowFeed(
	liveMarketDataProxy _interface.ILiveMarketDataProxy,
	clockProxy _interface.IClockProxy,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
	report func(context.Context, *kCandleFollowSymbol, vo.LiveKCandleVo),
) *kCandleFollowFeed {
	return &kCandleFollowFeed{
		liveMarketDataProxy: liveMarketDataProxy,
		clockProxy:          clockProxy,
		quietTimeout:        quietTimeout,
		maximumRetryDelay:   maximumRetryDelay,
		report:              report,
	}
}

// keep retries the line forever, telling viewers each time it stalls, until its context is ended on purpose.
func (kCandleFollowFeed *kCandleFollowFeed) keep(
	executionContext context.Context, openChannel *kCandleFollowChannel,
) {
	defer close(openChannel.finished)

	healthDomain := domains.NewLiveChannelHealthDomain(
		kCandleFollowFeed.quietTimeout,
		kCandleFollowFeed.maximumRetryDelay,
		kCandleFollowFeed.clockProxy.Now(),
	)

	for {
		// A per-attempt context ensures an abandoned (e.g. silent) connection is really closed before the next dial, so there is never more than one line.
		attemptContext, abandonAttempt := context.WithCancel(executionContext)

		liveKCandles, followError := kCandleFollowFeed.liveMarketDataProxy.
			FollowKCandles(attemptContext, openChannel.channel)
		if followError == nil {
			healthDomain.MarkConnected(kCandleFollowFeed.clockProxy.Now())
			kCandleFollowFeed.consume(attemptContext, openChannel, healthDomain, liveKCandles)
		} else {
			log.Printf("live k candle follow: %s could not be followed: %v",
				openChannel.channel.Key, followError)
		}

		abandonAttempt()

		// A done context means the channel was ended on purpose; saying "stalled" would promise a recovery and overwrite the real reason.
		if executionContext.Err() != nil {
			return
		}

		openChannel.publishStalled(nil)

		// Logged because the gap is the only evidence the back-off is working against a source that accepts then drops connections.
		retryDelay := healthDomain.NextRetryDelay()
		log.Printf("live k candle follow: %s is not delivering; trying again in %s",
			openChannel.channel.Key, retryDelay)

		if !kCandleFollowFeed.waitOrDone(executionContext, retryDelay) {
			return
		}
	}
}

// consume reads candles until the feed ends or goes quiet, since an open-looking but silent connection is this feed's usual failure mode.
// It is a separate method so the quiet-check ticker's defer Stop scopes it to one attempt instead of leaking one per outage.
func (kCandleFollowFeed *kCandleFollowFeed) consume(
	executionContext context.Context,
	openChannel *kCandleFollowChannel,
	healthDomain *domains.LiveChannelHealthDomain,
	liveKCandles <-chan vo.LiveKCandleVo,
) {
	quietCheck := time.NewTicker(healthDomain.QuietCheckInterval())
	defer quietCheck.Stop()

	for {
		select {
		case <-executionContext.Done():
			return

		case liveKCandle, isDelivering := <-liveKCandles:
			if !isDelivering {
				return
			}

			// Any candle proves the line is alive; silence belongs to the line, not to a symbol.
			healthDomain.MarkReceived(kCandleFollowFeed.clockProxy.Now())

			follow, isCarried := openChannel.followOf(liveKCandle.Symbol)
			if !isCarried {
				// Logged because a mismatched symbol name (case, suffix) otherwise looks exactly like a quiet market.
				log.Printf("live k candle follow: %s carries no %s, dropping its candle",
					openChannel.channel.Key, liveKCandle.Symbol)

				continue
			}
			kCandleFollowFeed.report(executionContext, follow, liveKCandle)

		case <-quietCheck.C:
			if healthDomain.HasGoneQuiet(kCandleFollowFeed.clockProxy.Now()) {
				return
			}
		}
	}
}

// waitOrDone waits out the retry gap, returning false if the follow ended first.
func (kCandleFollowFeed *kCandleFollowFeed) waitOrDone(
	executionContext context.Context, delay time.Duration,
) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-executionContext.Done():
		return false
	case <-timer.C:
		return true
	}
}

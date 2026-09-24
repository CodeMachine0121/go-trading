package service

import (
	"context"
	"log"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleFollowFeed keeps a live line followed against one live source: it dials the
// line, hands every candle that arrives to whoever owns the line, notices when the line
// has gone quiet, tells the line's viewers it has stalled, and tries again after a
// growing wait — until the line is ended on purpose.
//
// None of that depends on which market the line is in, so the spot follow and the
// contract follow each hold one of these rather than each keeping a copy of the loop.
// What a candle arriving amounts to — passed on, stored, or only passed on — is the
// owner's decision, handed in as report; how long silence and retries may last is
// LiveChannelHealthDomain's.
//
// It carries execution, not domain data — a source, a clock, three timing figures and
// the callback it reports to — so it is not a domain model and lives beside the
// services that run it, with no suffix.
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

// keep keeps one line followed for as long as anyone is watching it. Every time the
// feed ends — refused, dropped, or gone silent — the viewers are told, the wait
// grows, and it tries again. It never gives up; only the last viewer leaving ends it.
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
		// Each attempt gets a context of its own so that abandoning it really closes
		// the line behind it. A feed that fell silent is still open — its reader is
		// sitting on a socket nobody is listening to any more — and dialling the next
		// attempt without letting go of it would leave two lines where the plan
		// allows one, which is the very thing being followed a channel at a time was
		// meant to prevent.
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

		// A channel whose context is already done was ended on purpose — the system is
		// shutting down, or these symbols lost their places. Saying "stalled" then
		// would leave a viewer waiting for a recovery nobody intends, and would
		// overwrite the reason they were just given.
		if executionContext.Err() != nil {
			return
		}

		// One line went down, so every symbol on it hears the same news — nobody is
		// being retired here, the line is simply being tried again.
		openChannel.publishStalled(nil)

		// Said out loud because the gap is the only evidence the growing back-off is
		// working: a source that keeps accepting connections and dropping them is
		// otherwise indistinguishable, in the log, from one being retried every second.
		retryDelay := healthDomain.NextRetryDelay()
		log.Printf("live k candle follow: %s is not delivering; trying again in %s",
			openChannel.channel.Key, retryDelay)

		if !kCandleFollowFeed.waitOrDone(executionContext, retryDelay) {
			return
		}
	}
}

// consume decides when this follow has something to do: a candle arrived, the feed
// ended, or it has been silent long enough to count as dead.
//
// It is a method of its own rather than part of keep because it draws the stretch the
// quiet-check ticker is held for: one ticker per attempt, stopped the moment that
// attempt ends. Written inline, the stop would have to be remembered by hand at every
// way out of the loop, and a ticker left running per retry leaks one per outage.
//
// A connection that looks open but has stopped delivering is how this kind of feed
// usually fails, and a viewer must not be left watching a frozen picture that claims
// to be live.
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

			// Anything arriving proves the line is alive, whichever symbol it was
			// about — the silence and the retry gap belong to the line, not to a
			// symbol that happened to trade.
			healthDomain.MarkReceived(kCandleFollowFeed.clockProxy.Now())

			// The candle names the symbol it belongs to, and that is the only thing
			// that decides whose it is. Two symbols sharing a line are two pictures.
			follow, isCarried := openChannel.followOf(liveKCandle.Symbol)
			if !isCarried {
				// Said out loud because from every other angle this looks healthy:
				// the line is alive, candles are arriving, and not one of them ever
				// reaches a viewer. Silence here would make a name that does not
				// match — a case difference, a suffix — indistinguishable from a
				// market that simply has nothing to report.
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

// waitOrDone waits out the retry gap, reporting false if the follow ended first so
// that a follow nobody is watching stops immediately rather than after the wait.
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

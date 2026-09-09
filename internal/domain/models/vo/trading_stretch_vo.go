package vo

import "time"

// TradingStretchVo is one continuous run of trading inside a market's own day, said
// as offsets from that day's local midnight.
//
// A market is written down as a list of these rather than as one start and one end,
// because a venue may trade more than once a day — Taiwan index futures trades a day
// board and then an evening board — and a system that can only hold one of them has
// to call the other a fault.
//
// EndOffset may run past twenty-four hours, and that is how a stretch that crosses
// midnight is expressed: the evening board is 15:00 to 29:00 rather than 15:00 to
// 05:00 with a flag saying "but tomorrow". Written that way, ordering, length and
// containment are all still plain arithmetic, and nothing downstream has to remember
// which end belongs to which date.
//
// Immutable, no behavior beyond reading itself — which day's trading it counts
// towards, and whether a moment falls inside it, are MarketDomain's to answer.
type TradingStretchVo struct {
	// StartOffset is how far into the local day this stretch begins.
	StartOffset time.Duration
	// EndOffset is how far into the local day it ends, exclusive. Greater than
	// twenty-four hours means it runs into the following day.
	EndOffset time.Duration
	// BelongsToNextBusinessDay says this stretch's trading counts towards the next
	// business day rather than the day it starts on — the reading the exchange itself
	// takes of an evening board, so that a Friday evening's trading is Monday's.
	//
	// It is a property of the stretch rather than a rule about evenings, so that a
	// venue which numbers its boards differently is a different list of stretches and
	// not a new branch somewhere.
	BelongsToNextBusinessDay bool
}

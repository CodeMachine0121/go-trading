package domains

import (
	"cmp"
	"slices"
	"time"
)

// orderFlowDayLength is one UTC calendar day, the unit the venue's archive is published in.
const orderFlowDayLength = 24 * time.Hour

// Estimated in-memory footprint used to keep remembered order flow within its budget.
const (
	orderFlowLevelBytes  = 32
	orderFlowMinuteBytes = 48
	orderFlowDayBytes    = 128
)

// ContractOrderFlowDayDomain is one symbol's trades for one UTC day, tallied by minute at one price step, as far as they have been gathered; it never changes, so remembered days are shared safely and extending one yields a new value.
type ContractOrderFlowDayDomain struct {
	priceStep OrderFlowPriceStepDomain
	dayStart  time.Time
	// coveredUntil is exclusive: every minute before it is whole.
	coveredUntil time.Time
	// minutes are oldest first and hold only minutes that traded.
	minutes []orderFlowMinute
}

// orderFlowMinute holds one minute's traded levels, lowest first.
type orderFlowMinute struct {
	openUnix int64
	levels   []orderFlowLevel
}

type orderFlowLevel struct {
	level int64
	orderFlowTally
}

// NewContractOrderFlowDayDomain is a day nothing has been gathered for yet.
func NewContractOrderFlowDayDomain(priceStep OrderFlowPriceStepDomain, day time.Time) ContractOrderFlowDayDomain {
	dayStart := day.UTC().Truncate(orderFlowDayLength)

	return ContractOrderFlowDayDomain{priceStep: priceStep, dayStart: dayStart, coveredUntil: dayStart}
}

// Extending adds the ledger's whole minutes from where this day stopped up to coveredUntil (capped at the day's end); minutes already held are never replaced.
func (dayDomain ContractOrderFlowDayDomain) Extending(
	ledger *OrderFlowMinuteLedgerDomain, coveredUntil time.Time,
) ContractOrderFlowDayDomain {
	newCoveredUntil := dayDomain.NeededUntil(coveredUntil)
	if !newCoveredUntil.After(dayDomain.coveredUntil) {
		return dayDomain
	}

	addedMinutes := make([]orderFlowMinute, 0, len(ledger.tallies))
	for minuteOpen, tallies := range ledger.tallies {
		if minuteOpen < dayDomain.coveredUntil.Unix() || minuteOpen >= newCoveredUntil.Unix() {
			continue
		}

		levels := make([]orderFlowLevel, 0, len(tallies))
		for level, tally := range tallies {
			levels = append(levels, orderFlowLevel{level: level, orderFlowTally: *tally})
		}
		slices.SortFunc(levels, func(former, latter orderFlowLevel) int {
			return cmp.Compare(former.level, latter.level)
		})
		addedMinutes = append(addedMinutes, orderFlowMinute{openUnix: minuteOpen, levels: levels})
	}
	slices.SortFunc(addedMinutes, func(former, latter orderFlowMinute) int {
		return cmp.Compare(former.openUnix, latter.openUnix)
	})

	extended := dayDomain
	extended.minutes = append(slices.Clip(dayDomain.minutes), addedMinutes...)
	extended.coveredUntil = newCoveredUntil

	return extended
}

// ReadsFromArchive is true for a day that has ended and has nothing gathered yet; a day already partly read live carries on live, so its minutes never come from two sources.
func (dayDomain ContractOrderFlowDayDomain) ReadsFromArchive(now time.Time) bool {
	return dayDomain.dayStart.Before(now.UTC().Truncate(orderFlowDayLength)) && !dayDomain.coveredUntil.After(dayDomain.dayStart)
}

// ArchiveMayStillBeComing is true only for yesterday: the venue publishes a day's archive about a day late, so a missing file then is expected and the day is read live instead; older missing files are a failure.
func (dayDomain ContractOrderFlowDayDomain) ArchiveMayStillBeComing(now time.Time) bool {
	return !dayDomain.dayStart.Before(now.UTC().Truncate(orderFlowDayLength).Add(-orderFlowDayLength))
}

// Covers reports whether every minute up to until (or the whole day, if until is later) is held.
func (dayDomain ContractOrderFlowDayDomain) Covers(until time.Time) bool {
	return !dayDomain.coveredUntil.Before(dayDomain.NeededUntil(until))
}

func (dayDomain ContractOrderFlowDayDomain) CoveredUntil() time.Time {
	return dayDomain.coveredUntil
}

func (dayDomain ContractOrderFlowDayDomain) PriceStep() OrderFlowPriceStepDomain {
	return dayDomain.priceStep
}

func (dayDomain ContractOrderFlowDayDomain) DayStart() time.Time {
	return dayDomain.dayStart
}

// EstimatedBytes approximates the memory this day holds, for the remembered order flow's budget.
func (dayDomain ContractOrderFlowDayDomain) EstimatedBytes() int64 {
	estimatedBytes := int64(orderFlowDayBytes)
	for _, minute := range dayDomain.minutes {
		estimatedBytes += orderFlowMinuteBytes + int64(len(minute.levels))*orderFlowLevelBytes
	}

	return estimatedBytes
}

// NeededUntil keeps a moment inside this day: how far into the day a reach ending then needs trades, since a day never holds minutes of the next.
func (dayDomain ContractOrderFlowDayDomain) NeededUntil(moment time.Time) time.Time {
	dayEnd := dayDomain.dayStart.Add(orderFlowDayLength)
	if moment.After(dayEnd) {
		return dayEnd
	}

	return moment.UTC()
}

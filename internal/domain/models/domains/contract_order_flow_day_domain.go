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
	newCoveredUntil := dayDomain.cappedAtDayEnd(coveredUntil)
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

// Covers reports whether every minute up to until (or the whole day, if until is later) is held.
func (dayDomain ContractOrderFlowDayDomain) Covers(until time.Time) bool {
	return !dayDomain.coveredUntil.Before(dayDomain.cappedAtDayEnd(until))
}

func (dayDomain ContractOrderFlowDayDomain) CoveredUntil() time.Time {
	return dayDomain.coveredUntil
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

// cappedAtDayEnd keeps a moment inside this day, since a day never holds minutes of the next.
func (dayDomain ContractOrderFlowDayDomain) cappedAtDayEnd(moment time.Time) time.Time {
	dayEnd := dayDomain.dayStart.Add(orderFlowDayLength)
	if moment.After(dayEnd) {
		return dayEnd
	}

	return moment.UTC()
}

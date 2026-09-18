package domains

import "errors"

// positionSizingFailure is a declaration of how much one opening stakes that could
// not be read, carrying the sentence and one fact about it: whether the figure beside
// the mode is what was wrong.
//
// Its Error is the sentence alone, with no sentinel of its own. Two worlds ask this
// question now — a replay being set up and a bot being saved — and each has to wrap
// the same sentence in the sentinel its own controller already maps. A sentinel here
// would end up printed inside the other's message, and a bot refused for a percentage
// of a hundred and fifty would tell its owner that a backtest had failed.
//
// Which of the two things went wrong travels as a value rather than as prose, for the
// reason the field name on a replay's refusal does: matching on the sentence would be
// matching on words written for a person, and those change whenever the wording
// improves.
type positionSizingFailure struct {
	reason      string
	aboutFigure bool
}

func (failure *positionSizingFailure) Error() string {
	return failure.reason
}

// positionSizingModeFailure is a mode nobody offers.
func positionSizingModeFailure(reason string) error {
	return &positionSizingFailure{reason: reason}
}

// positionSizingFigureFailure is a figure the mode beside it cannot use.
func positionSizingFigureFailure(reason string) error {
	return &positionSizingFailure{reason: reason, aboutFigure: true}
}

// PositionSizingFailureAboutFigure says whether this refusal is about the figure
// rather than the mode.
//
// A replay needs to tell them apart because only one of them names an input a caller
// can go and change; a bot does not, and asks nothing.
func PositionSizingFailureAboutFigure(err error) bool {
	var failure *positionSizingFailure
	if !errors.As(err, &failure) {
		return false
	}

	return failure.aboutFigure
}

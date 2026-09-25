package domains

import "errors"

// positionSizingFailure carries only the sentence, with no sentinel, so replays and bots can each wrap it in their own; aboutFigure travels as a value so callers need not parse prose.
type positionSizingFailure struct {
	reason      string
	aboutFigure bool
}

func (failure *positionSizingFailure) Error() string {
	return failure.reason
}

func positionSizingModeFailure(reason string) error {
	return &positionSizingFailure{reason: reason}
}

func positionSizingFigureFailure(reason string) error {
	return &positionSizingFailure{reason: reason, aboutFigure: true}
}

// PositionSizingFailureAboutFigure lets a replay tell a bad figure from a bad mode.
func PositionSizingFailureAboutFigure(err error) bool {
	var failure *positionSizingFailure
	if !errors.As(err, &failure) {
		return false
	}

	return failure.aboutFigure
}

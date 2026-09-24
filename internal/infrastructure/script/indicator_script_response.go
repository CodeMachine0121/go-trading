package script

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptResponse is everything a compartment says back: the values of a single
// run, the values of every run in a replay, or why it failed. Exactly one of those is
// filled in.
type indicatorScriptResponse struct {
	Values           indicatorValuesWire
	PerElementValues []indicatorValuesWire
	// FailureMessage is the failure exactly as the compartment worded it, so that the
	// author reads the same sentence whichever side of the boundary it was written on.
	FailureMessage string
	HasFailure     bool
	// UndeclaredParameterName is kept apart from FailureMessage because the caller
	// hands it over as a field of its own; recovering it from prose would break the
	// day the wording improved.
	UndeclaredParameterName string
	HasUndeclaredParameter  bool
}

// failure rebuilds the error the compartment ran into, as the kind the rest of the
// service tells failures apart by. Nil means the run succeeded.
func (response indicatorScriptResponse) failure() error {
	if response.HasUndeclaredParameter {
		return domains.UndeclaredParameter(response.UndeclaredParameterName)
	}
	if !response.HasFailure {
		return nil
	}

	// The message already begins with the script-failure prefix, because it was
	// written by wrapping the same sentinel on the other side. Wrapping it again as
	// it stands would say "indicator script failed" twice.
	return fmt.Errorf("%w%s", domains.ErrIndicatorScriptFailed,
		strings.TrimPrefix(response.FailureMessage, domains.ErrIndicatorScriptFailed.Error()))
}

// values reads back the values of a single run.
func (response indicatorScriptResponse) values() map[string]vo.IndicatorValueVo {
	return response.Values.toIndicatorValues()
}

// perElementValues reads back the values of every run in a replay, in order.
func (response indicatorScriptResponse) perElementValues() []map[string]vo.IndicatorValueVo {
	perElementIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(response.PerElementValues))
	for _, elementValues := range response.PerElementValues {
		perElementIndicatorValues = append(perElementIndicatorValues, elementValues.toIndicatorValues())
	}

	return perElementIndicatorValues
}

// newFailedIndicatorScriptResponse is how a compartment reports an error it ran into. An
// undeclared knob is reported by name; anything else by its message.
func newFailedIndicatorScriptResponse(runError error) indicatorScriptResponse {
	if parameterName, isUndeclared := domains.UndeclaredParameterName(runError); isUndeclared {
		return indicatorScriptResponse{UndeclaredParameterName: parameterName, HasUndeclaredParameter: true}
	}
	if !errors.Is(runError, domains.ErrIndicatorScriptFailed) {
		runError = fmt.Errorf("%w: %v", domains.ErrIndicatorScriptFailed, runError)
	}

	return indicatorScriptResponse{FailureMessage: runError.Error(), HasFailure: true}
}

package script

import (
	"errors"
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptResponse holds exactly one of single-run values, replay values, or a failure.
type indicatorScriptResponse struct {
	Values           indicatorValuesWire
	PerElementValues []indicatorValuesWire
	FailureMessage   string
	HasFailure       bool
	// UndeclaredParameterName is a separate field so callers never parse it out of the message.
	UndeclaredParameterName string
	HasUndeclaredParameter  bool
}

// failure rebuilds the compartment's error as the service's sentinel; nil means success.
func (response indicatorScriptResponse) failure() error {
	if response.HasUndeclaredParameter {
		return domains.UndeclaredParameter(response.UndeclaredParameterName)
	}
	if !response.HasFailure {
		return nil
	}

	// The message already starts with the sentinel's prefix, so it is not wrapped a second time.
	return fmt.Errorf("%w%s", domains.ErrIndicatorScriptFailed,
		strings.TrimPrefix(response.FailureMessage, domains.ErrIndicatorScriptFailed.Error()))
}

func (response indicatorScriptResponse) values() map[string]vo.IndicatorValueVo {
	return response.Values.toIndicatorValues()
}

func (response indicatorScriptResponse) perElementValues() []map[string]vo.IndicatorValueVo {
	perElementIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(response.PerElementValues))
	for _, elementValues := range response.PerElementValues {
		perElementIndicatorValues = append(perElementIndicatorValues, elementValues.toIndicatorValues())
	}

	return perElementIndicatorValues
}

func newFailedIndicatorScriptResponse(runError error) indicatorScriptResponse {
	if parameterName, isUndeclared := domains.UndeclaredParameterName(runError); isUndeclared {
		return indicatorScriptResponse{UndeclaredParameterName: parameterName, HasUndeclaredParameter: true}
	}
	if !errors.Is(runError, domains.ErrIndicatorScriptFailed) {
		runError = fmt.Errorf("%w: %v", domains.ErrIndicatorScriptFailed, runError)
	}

	return indicatorScriptResponse{FailureMessage: runError.Error(), HasFailure: true}
}

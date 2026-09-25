package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// RunSubjectDomain is exactly one of a saved strategy script the caller named or an algorithm they supplied, never both and never neither.
type RunSubjectDomain struct {
	strategyScriptID uint
	ownScript        string
	resultType       string
	parameters       []dto.StrategyScriptParameterWriteDto
}

// NewRunSubjectDomain takes the result type and parameters here because they belong to a supplied algorithm; a named script's own are used instead.
func NewRunSubjectDomain(
	strategyScriptID uint,
	ownScript string,
	resultType string,
	parameters []dto.StrategyScriptParameterWriteDto,
) (RunSubjectDomain, error) {
	namesAStrategyScript := strategyScriptID != 0
	carriesAnAlgorithm := ownScript != ""

	if namesAStrategyScript && carriesAnAlgorithm {
		return RunSubjectDomain{}, fmt.Errorf(
			"%w: 指名一支策略腳本與自帶一段算式只能挑一種——兩個都給，說不出實際跑的是哪一個",
			ErrRunSubjectAmbiguous)
	}

	if !namesAStrategyScript && !carriesAnAlgorithm {
		return RunSubjectDomain{}, fmt.Errorf(
			"%w: 要嘛指名一支策略腳本，要嘛帶一段算式，沒有第三種跑法",
			ErrRunSubjectAmbiguous)
	}

	return RunSubjectDomain{
		strategyScriptID: strategyScriptID,
		ownScript:        ownScript,
		resultType:       resultType,
		parameters:       parameters,
	}, nil
}

// NamedStrategyScriptID returns false when the caller supplied its own algorithm instead.
func (runSubjectDomain RunSubjectDomain) NamedStrategyScriptID() (uint, bool) {
	return runSubjectDomain.strategyScriptID, runSubjectDomain.strategyScriptID != 0
}

// ToRunnableDto shapes the supplied algorithm like a resolved strategy script; it is only asked for after NamedStrategyScriptID returned false.
func (runSubjectDomain RunSubjectDomain) ToRunnableDto() dto.RunnableStrategyScriptDto {
	return dto.RunnableStrategyScriptDto{
		Script:     runSubjectDomain.ownScript,
		ResultType: runSubjectDomain.resultType,
		Parameters: runSubjectDomain.parameters,
		// A caller's own algorithm is their own words.
		OwnedByViewer: true,
	}
}

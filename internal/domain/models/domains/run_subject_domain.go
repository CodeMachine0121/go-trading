package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// RunSubjectDomain is what a run is about to execute: a saved strategy script the caller
// named, or an algorithm the caller wrote themselves.
//
// Both exist, and both are correct. Naming a strategy script is what lets one person run
// another's published algorithm without ever being handed it. Carrying one is what
// lets a person try something they just typed — and that hides nothing, because it
// is their own text: the rule this feature protects is that nobody receives an
// algorithm they may not read, not that nobody may supply one.
//
// What the two must never be is both at once. Sent together they can disagree about
// what actually ran, and picking a winner is a decision nobody asked for; sent
// neither way there is nothing to run. This model is where that is settled, once,
// for every use case that runs something.
type RunSubjectDomain struct {
	strategyScriptID uint
	ownScript        string
	resultType       string
	parameters       []dto.StrategyScriptParameterWriteDto
}

// NewRunSubjectDomain reads what the caller named and what they carried, and
// refuses anything that is not exactly one of the two.
//
// The kind of value and the knobs come in here rather than being passed alongside
// later, because they belong to the algorithm the caller carried. A named strategy script
// declared its own, and these are then never read — which is why the caller hands
// them over once, here, instead of every use case taking an extra argument it uses
// half the time.
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

// NamedStrategyScriptID is the strategy script to resolve, and whether one was named at all. A
// caller that gets false carried its own algorithm instead.
func (runSubjectDomain RunSubjectDomain) NamedStrategyScriptID() (uint, bool) {
	return runSubjectDomain.strategyScriptID, runSubjectDomain.strategyScriptID != 0
}

// ToRunnableDto is the caller's own algorithm as something to run, in the same
// shape a resolved strategy script arrives in — so what runs it never has to know which
// of the two it got.
//
// A subject that named a strategy script has nothing to give here, and says so with an
// empty algorithm rather than a refusal: the caller asks for this only after the
// model has told it no strategy script was named.
func (runSubjectDomain RunSubjectDomain) ToRunnableDto() dto.RunnableStrategyScriptDto {
	return dto.RunnableStrategyScriptDto{
		Script:     runSubjectDomain.ownScript,
		ResultType: runSubjectDomain.resultType,
		Parameters: runSubjectDomain.parameters,
	}
}

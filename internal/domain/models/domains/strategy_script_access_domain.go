package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// StrategyScriptAccessDomain answers what one strategy script is to one viewer: theirs to read
// and change, somebody else's but out on the marketplace, or — as far as they are
// concerned — not there at all.
//
// Every path that reaches a strategy script builds one of these and asks it. That is the
// point of the type: the rule lives in one place, so there is no second place to
// get it wrong, and adding a third kind of visibility later means teaching this
// model one more fact rather than editing every service that reads a strategy script.
//
// # The three gates
//
//  1. Does the strategy script exist?
//  2. Is it the viewer's own? — if so, through.
//  3. If not, is it published? — if so, through; otherwise stop.
//
// The first gate is not here, and cannot be: a model holding a strategy script is a model
// whose strategy script exists. The store answers that one, with ErrStrategyScriptNotFound —
// which is the same refusal this model returns for the other two. Sharing one error
// is what makes "not there" and "not yours" indistinguishable; two spellings of it
// would be two sentences somebody has to keep identical, and a caller holding a
// list of identifiers could tell them apart and learn which ones exist.
//
// Reading and running walk all three gates. Changing, deleting, publishing and
// withdrawing stop at the second: publishing hands out the use of a strategy script, never
// the right to alter it.
type StrategyScriptAccessDomain struct {
	strategyScript entities.StrategyScript
	viewerID       uint
	isPublished    bool
}

// NewStrategyScriptAccessDomain takes the strategy script, who is asking, and whether it is on
// the marketplace. Those three facts are everything the decision needs, so there is
// no half-built state and nothing to look up later.
func NewStrategyScriptAccessDomain(
	strategyScript entities.StrategyScript, viewerID uint, isPublished bool,
) StrategyScriptAccessDomain {
	return StrategyScriptAccessDomain{strategyScript: strategyScript, viewerID: viewerID, isPublished: isPublished}
}

// IsOwnedByViewer is the second gate: this strategy script belongs to whoever is asking.
//
// A viewer of zero is nobody, and nobody owns nothing. Saying so here rather than
// trusting the caller to have checked matters because a strategy script whose owner column
// somehow held zero would otherwise belong to every unauthenticated request at once.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) IsOwnedByViewer() bool {
	return strategyScriptAccessDomain.viewerID != 0 &&
		strategyScriptAccessDomain.strategyScript.OwnerID == strategyScriptAccessDomain.viewerID
}

// IsRunnable says whether this viewer may run the strategy script: their own, or anybody's
// that is out on the marketplace.
//
// Having adopted it does not come into it. Adoption decides what appears in a
// picker, not what may be run — otherwise nobody could try a strategy script before taking
// it, and choosing from the marketplace would be choosing blind.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) IsRunnable() bool {
	return strategyScriptAccessDomain.IsOwnedByViewer() || strategyScriptAccessDomain.isPublished
}

// ToOwnerDto hands over the strategy script in full, script included, and refuses anybody
// but the owner with the one refusal every closed door here gives.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) ToOwnerDto() (dto.StrategyScriptDto, error) {
	if !strategyScriptAccessDomain.IsOwnedByViewer() {
		return dto.StrategyScriptDto{}, StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
	}

	return strategyScriptAccessDomain.strategyScript.ToDto(), nil
}

// RequireOwnership is the second gate on its own, for the paths that change
// something and therefore have nothing to hand back on the way in: rewriting,
// deleting, publishing, withdrawing.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) RequireOwnership() error {
	if !strategyScriptAccessDomain.IsOwnedByViewer() {
		return StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
	}

	return nil
}

// ToRunnableDto resolves the strategy script into the three things a run needs, and
// refuses anyone the three gates turn away.
//
// This is the only way a script leaves storage for a run, and it goes to the
// application layer, never to a response. That is what makes running somebody
// else's strategy script possible without reading it.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) ToRunnableDto() (dto.RunnableStrategyScriptDto, error) {
	if !strategyScriptAccessDomain.IsRunnable() {
		return dto.RunnableStrategyScriptDto{}, StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
	}

	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(strategyScriptAccessDomain.strategyScript.Parameters))
	for _, parameter := range strategyScriptAccessDomain.strategyScript.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, dto.StrategyScriptParameterWriteDto{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			DefaultValue: parameter.DefaultValue,
		})
	}

	return dto.RunnableStrategyScriptDto{
		Script:     strategyScriptAccessDomain.strategyScript.Script,
		ResultType: strategyScriptAccessDomain.strategyScript.ResultType,
		Parameters: parameterWriteDtos,
	}, nil
}

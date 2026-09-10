package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// StrategyAccessDomain answers what one strategy is to one viewer: theirs to read
// and change, somebody else's but out on the marketplace, or — as far as they are
// concerned — not there at all.
//
// Every path that reaches a strategy builds one of these and asks it. That is the
// point of the type: the rule lives in one place, so there is no second place to
// get it wrong, and adding a third kind of visibility later means teaching this
// model one more fact rather than editing every service that reads a strategy.
//
// # The three gates
//
//  1. Does the strategy exist?
//  2. Is it the viewer's own? — if so, through.
//  3. If not, is it published? — if so, through; otherwise stop.
//
// The first gate is not here, and cannot be: a model holding a strategy is a model
// whose strategy exists. The store answers that one, with ErrStrategyNotFound —
// which is the same refusal this model returns for the other two. Sharing one error
// is what makes "not there" and "not yours" indistinguishable; two spellings of it
// would be two sentences somebody has to keep identical, and a caller holding a
// list of identifiers could tell them apart and learn which ones exist.
//
// Reading and running walk all three gates. Changing, deleting, publishing and
// withdrawing stop at the second: publishing hands out the use of a strategy, never
// the right to alter it.
type StrategyAccessDomain struct {
	strategy    entities.Strategy
	viewerID    uint
	isPublished bool
}

// NewStrategyAccessDomain takes the strategy, who is asking, and whether it is on
// the marketplace. Those three facts are everything the decision needs, so there is
// no half-built state and nothing to look up later.
func NewStrategyAccessDomain(
	strategy entities.Strategy, viewerID uint, isPublished bool,
) StrategyAccessDomain {
	return StrategyAccessDomain{strategy: strategy, viewerID: viewerID, isPublished: isPublished}
}

// IsOwnedByViewer is the second gate: this strategy belongs to whoever is asking.
//
// A viewer of zero is nobody, and nobody owns nothing. Saying so here rather than
// trusting the caller to have checked matters because a strategy whose owner column
// somehow held zero would otherwise belong to every unauthenticated request at once.
func (strategyAccessDomain StrategyAccessDomain) IsOwnedByViewer() bool {
	return strategyAccessDomain.viewerID != 0 &&
		strategyAccessDomain.strategy.OwnerID == strategyAccessDomain.viewerID
}

// IsRunnable says whether this viewer may run the strategy: their own, or anybody's
// that is out on the marketplace.
//
// Having adopted it does not come into it. Adoption decides what appears in a
// picker, not what may be run — otherwise nobody could try a strategy before taking
// it, and choosing from the marketplace would be choosing blind.
func (strategyAccessDomain StrategyAccessDomain) IsRunnable() bool {
	return strategyAccessDomain.IsOwnedByViewer() || strategyAccessDomain.isPublished
}

// ToOwnerDto hands over the strategy in full, script included, and refuses anybody
// but the owner with the one refusal every closed door here gives.
func (strategyAccessDomain StrategyAccessDomain) ToOwnerDto() (dto.StrategyDto, error) {
	if !strategyAccessDomain.IsOwnedByViewer() {
		return dto.StrategyDto{}, StrategyNotFound(strategyAccessDomain.strategy.ID)
	}

	return strategyAccessDomain.strategy.ToDto(), nil
}

// RequireOwnership is the second gate on its own, for the paths that change
// something and therefore have nothing to hand back on the way in: rewriting,
// deleting, publishing, withdrawing.
func (strategyAccessDomain StrategyAccessDomain) RequireOwnership() error {
	if !strategyAccessDomain.IsOwnedByViewer() {
		return StrategyNotFound(strategyAccessDomain.strategy.ID)
	}

	return nil
}

// ToRunnableDto resolves the strategy into the three things a run needs, and
// refuses anyone the three gates turn away.
//
// This is the only way a script leaves storage for a run, and it goes to the
// application layer, never to a response. That is what makes running somebody
// else's strategy possible without reading it.
func (strategyAccessDomain StrategyAccessDomain) ToRunnableDto() (dto.RunnableStrategyDto, error) {
	if !strategyAccessDomain.IsRunnable() {
		return dto.RunnableStrategyDto{}, StrategyNotFound(strategyAccessDomain.strategy.ID)
	}

	parameterWriteDtos := make([]dto.StrategyParameterWriteDto, 0, len(strategyAccessDomain.strategy.Parameters))
	for _, parameter := range strategyAccessDomain.strategy.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, dto.StrategyParameterWriteDto{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			DefaultValue: parameter.DefaultValue,
		})
	}

	return dto.RunnableStrategyDto{
		Script:     strategyAccessDomain.strategy.Script,
		ResultType: strategyAccessDomain.strategy.ResultType,
		Parameters: parameterWriteDtos,
	}, nil
}

// ToPublishedDto is the strategy as the marketplace shows it — everything but the
// algorithm, plus who put it there and when.
//
// It asks no permission, because there is none to ask: a published strategy is
// public, and this shape carries nothing that is not. The caller supplies the
// moment because that fact lives on the publication, not on the strategy.
func (strategyAccessDomain StrategyAccessDomain) ToPublishedDto(publishedAt time.Time) dto.PublishedStrategyDto {
	return dto.PublishedStrategyDto{
		ID:             strategyAccessDomain.strategy.ID,
		Name:           strategyAccessDomain.strategy.Name,
		Description:    strategyAccessDomain.strategy.Description,
		ResultType:     strategyAccessDomain.strategy.ResultType,
		PublisherEmail: strategyAccessDomain.strategy.Owner.Email,
		PublishedAt:    publishedAt.UTC(),
		Parameters:     strategyAccessDomain.strategy.ToParameterDtos(),
	}
}

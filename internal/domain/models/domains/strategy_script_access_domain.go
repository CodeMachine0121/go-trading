package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// StrategyScriptAccessDomain is the single access rule for a strategy script: read/run if owned or published, change/delete/publish/withdraw only if owned, and every refusal is ErrStrategyScriptNotFound so "not yours" is indistinguishable from "not there".
type StrategyScriptAccessDomain struct {
	strategyScript entities.StrategyScript
	viewerID       uint
	isPublished    bool
}

func NewStrategyScriptAccessDomain(
	strategyScript entities.StrategyScript, viewerID uint, isPublished bool,
) StrategyScriptAccessDomain {
	return StrategyScriptAccessDomain{strategyScript: strategyScript, viewerID: viewerID, isPublished: isPublished}
}

// IsOwnedByViewer never matches viewer zero, so a script with a zero owner can't belong to every unauthenticated request.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) IsOwnedByViewer() bool {
	return strategyScriptAccessDomain.viewerID != 0 &&
		strategyScriptAccessDomain.strategyScript.OwnerID == strategyScriptAccessDomain.viewerID
}

// IsAdoptedFromMarketplace tells a copy made on adoption from the owner's own work.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) IsAdoptedFromMarketplace() bool {
	return strategyScriptAccessDomain.strategyScript.IsAdoptedFromMarketplace
}

// RequireRewritable guards rewriting and publishing: only the owner's own work, never a marketplace copy, whose
// algorithm stays its author's.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) RequireRewritable(refusal error) error {
	if ownershipError := strategyScriptAccessDomain.RequireOwnership(); ownershipError != nil {
		return ownershipError
	}
	if strategyScriptAccessDomain.IsAdoptedFromMarketplace() {
		return refusal
	}

	return nil
}

// ToOwnedRunnableDto is the gate for anything a bot can depend on: only the viewer's own scripts, copies included;
// someone else's published script is refused with a way forward, anything else reads as missing.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) ToOwnedRunnableDto() (dto.RunnableStrategyScriptDto, error) {
	if strategyScriptAccessDomain.IsOwnedByViewer() {
		return strategyScriptAccessDomain.ToRunnableDto()
	}
	if strategyScriptAccessDomain.isPublished {
		return dto.RunnableStrategyScriptDto{}, StrategyScriptNotYours(strategyScriptAccessDomain.strategyScript.ID)
	}

	return dto.RunnableStrategyScriptDto{}, StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
}

// IsRunnable allows owned or published scripts, so a marketplace script can be tried before it is adopted.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) IsRunnable() bool {
	return strategyScriptAccessDomain.IsOwnedByViewer() || strategyScriptAccessDomain.isPublished
}

// ToOwnerDto returns the full script, including source, to its owner only.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) ToOwnerDto() (dto.StrategyScriptDto, error) {
	if !strategyScriptAccessDomain.IsOwnedByViewer() {
		return dto.StrategyScriptDto{}, StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
	}

	return strategyScriptAccessDomain.strategyScript.ToDto(), nil
}

// RequireOwnership guards the mutating paths: rewrite, delete, publish, withdraw.
func (strategyScriptAccessDomain StrategyScriptAccessDomain) RequireOwnership() error {
	if !strategyScriptAccessDomain.IsOwnedByViewer() {
		return StrategyScriptNotFound(strategyScriptAccessDomain.strategyScript.ID)
	}

	return nil
}

// ToRunnableDto is the only way script source leaves storage for a run, and it goes to the application layer, never a response.
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
		Script:           strategyScriptAccessDomain.strategyScript.Script,
		ResultType:       strategyScriptAccessDomain.strategyScript.ResultType,
		MarketDataKind:   strategyScriptAccessDomain.strategyScript.MarketDataKind,
		Parameters:       parameterWriteDtos,
		AuthoredByViewer: strategyScriptAccessDomain.IsOwnedByViewer() && !strategyScriptAccessDomain.IsAdoptedFromMarketplace(),
	}, nil
}

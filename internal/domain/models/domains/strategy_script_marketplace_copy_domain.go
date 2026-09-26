package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// marketplaceCopyNameMark tells an adopter's copy apart from their own script of the same name when a copy has to be renamed.
const marketplaceCopyNameMark = "市集"

// StrategyScriptMarketplaceCopyDomain builds the snapshot an adopter receives: everything the original is at that
// moment, owned by the adopter and marked as adopted, with no link back.
type StrategyScriptMarketplaceCopyDomain struct {
	original   entities.StrategyScript
	adopterID  uint
	adoptedAt  time.Time
	copiedName string
}

// NewStrategyScriptMarketplaceCopyDomain expects the original with its parameters loaded.
func NewStrategyScriptMarketplaceCopyDomain(
	original entities.StrategyScript, adopterID uint, adoptedAt time.Time,
) StrategyScriptMarketplaceCopyDomain {
	return StrategyScriptMarketplaceCopyDomain{
		original: original, adopterID: adopterID, adoptedAt: adoptedAt, copiedName: original.Name,
	}
}

// NamedAvoiding marks the copy's name, then numbers it, until it clashes with none the adopter holds; it is for
// moving existing adoptions, where refusing is not an option.
func (strategyScriptMarketplaceCopyDomain StrategyScriptMarketplaceCopyDomain) NamedAvoiding(
	takenNames map[string]bool,
) StrategyScriptMarketplaceCopyDomain {
	named := strategyScriptMarketplaceCopyDomain
	baseName := strategyScriptMarketplaceCopyDomain.original.Name

	for attempt := 1; takenNames[named.copiedName]; attempt++ {
		named.copiedName = fmt.Sprintf("%s（%s）", baseName, marketplaceCopyNameMark)
		if attempt > 1 {
			named.copiedName = fmt.Sprintf("%s（%s %d）", baseName, marketplaceCopyNameMark, attempt)
		}
	}

	return named
}

func (strategyScriptMarketplaceCopyDomain StrategyScriptMarketplaceCopyDomain) ToEntity() entities.StrategyScript {
	original := strategyScriptMarketplaceCopyDomain.original

	parameters := make([]entities.StrategyScriptParameter, 0, len(original.Parameters))
	for _, parameter := range original.Parameters {
		parameters = append(parameters, entities.StrategyScriptParameter{
			Name:         parameter.Name,
			Kind:         parameter.Kind,
			DefaultValue: parameter.DefaultValue,
		})
	}

	adoptedAt := strategyScriptMarketplaceCopyDomain.adoptedAt.UTC()

	return entities.StrategyScript{
		OwnerID:                  strategyScriptMarketplaceCopyDomain.adopterID,
		Name:                     strategyScriptMarketplaceCopyDomain.copiedName,
		Description:              original.Description,
		Script:                   original.Script,
		ResultType:               original.ResultType,
		MarketDataKind:           original.MarketDataKind,
		IsAdoptedFromMarketplace: true,
		CreatedAt:                adoptedAt,
		UpdatedAt:                adoptedAt,
		Parameters:               parameters,
	}
}

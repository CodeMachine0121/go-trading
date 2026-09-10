package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// strategyNameMaxLength is how long a strategy name may be, counted after the blanks
// around it are dropped. It is the single place this limit is written down.
const strategyNameMaxLength = 128

// strategyDescriptionMaxLength is how long a strategy's description may be, counted
// after the blanks around it are dropped. It is generous because on the marketplace
// it is the only thing a reader has to go on — the script is never handed out — but
// it is bounded because a description is a paragraph, not a document.
const strategyDescriptionMaxLength = 512

// StrategyDomain holds one strategy and guarantees its own invariants. An instance
// only exists when every rule passed, so there is no half-valid strategy.
//
// A strategy is an algorithm and nothing more: a name, a script and the kind of
// value that script produces. It holds no plan for feeding itself — how coarse the
// K candles are, how many of them, and up to when are settled by whoever runs it,
// so one algorithm can be run at any coarseness over any stretch of market instead
// of being saved once per way of looking at it.
//
// The result type already has a model that knows what it may be; this one borrows it
// rather than listing its values a second time. What it adds is the rest of the
// rules and a single kind of refusal, so that a caller never has to recognise a K
// candle's sentinel to find out its strategy was rejected.
//
// It deliberately does not look at the script beyond "there is one". A script that
// cannot run is still worth saving: an algorithm usually takes several sittings to
// get right, and a strategy that refused to be saved half-finished could not be
// picked up again tomorrow.
type StrategyDomain struct {
	id          uint
	ownerID     uint
	name        string
	description string
	script      string
	resultType  IndicatorResultTypeDomain
	parameters  StrategyParametersDomain
}

// NewStrategyDomain validates the strategy against every rule that applies to it.
// The rules are identical whether the strategy is being created or rewritten,
// because both arrive here as the same shape.
func NewStrategyDomain(writeDto dto.StrategyWriteDto) (StrategyDomain, error) {
	// A strategy with nobody behind it is refused here rather than at the store,
	// because "every strategy has an owner" is a rule about strategies, not a
	// constraint that happens to exist on a column. Written here, it holds for
	// anything that ever builds one — including a future caller that does not go
	// through the same store.
	if writeDto.OwnerID == 0 {
		return StrategyDomain{}, fmt.Errorf("%w: 策略必須屬於一位使用者", ErrStrategyValidation)
	}

	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return StrategyDomain{}, fmt.Errorf("%w: 必須給策略取一個名稱", ErrStrategyValidation)
	}

	if strings.ContainsRune(name, nulCharacter) {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略名稱不得包含空字元（NUL）", ErrStrategyValidation)
	}

	if len([]rune(name)) > strategyNameMaxLength {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略名稱長度上限為 %d 個字", ErrStrategyValidation, strategyNameMaxLength)
	}

	// A description is optional, so blanks and nothing are the same thing: somebody
	// who typed only spaces said nothing, and storing their spaces would make the
	// marketplace show an empty paragraph instead of no paragraph.
	description := strings.TrimSpace(writeDto.Description)
	if len([]rune(description)) > strategyDescriptionMaxLength {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略說明長度上限為 %d 個字", ErrStrategyValidation, strategyDescriptionMaxLength)
	}

	if strings.ContainsRune(description, nulCharacter) {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略說明不得包含空字元（NUL）", ErrStrategyValidation)
	}

	if strings.TrimSpace(writeDto.Script) == "" {
		return StrategyDomain{}, fmt.Errorf("%w: 策略必須帶一段指標算式", ErrStrategyValidation)
	}

	if strings.ContainsRune(writeDto.Script, nulCharacter) {
		return StrategyDomain{}, fmt.Errorf(
			"%w: 策略算式不得包含空字元（NUL）", ErrStrategyValidation)
	}

	resultType, resultTypeError := NewIndicatorResultTypeDomain(writeDto.ResultType)
	if resultTypeError != nil {
		return StrategyDomain{}, fmt.Errorf("%w: %w", ErrStrategyValidation, resultTypeError)
	}

	// The knobs already have a model that knows every rule about them; this one
	// borrows it rather than restating those rules a second time.
	parameters, parametersError := NewStrategyParametersDomain(writeDto.Parameters)
	if parametersError != nil {
		return StrategyDomain{}, fmt.Errorf("%w: %w", ErrStrategyValidation, parametersError)
	}

	return StrategyDomain{
		id:          writeDto.ID,
		ownerID:     writeDto.OwnerID,
		name:        name,
		description: description,
		script:      writeDto.Script,
		resultType:  resultType,
		parameters:  parameters,
	}, nil
}

// ResultType is the kind of value this strategy produces, already read and accepted.
// It is handed out as the kind itself rather than as its spelling, so that a
// declaration is only ever interpreted once.
func (strategyDomain StrategyDomain) ResultType() IndicatorResultTypeDomain {
	return strategyDomain.resultType
}

// ToEntity is the strategy as it is stored. The times are left alone: when a
// strategy was first saved and when it was last touched are recorded where the
// saving happens, not claimed here.
func (strategyDomain StrategyDomain) ToEntity() entities.Strategy {
	return entities.Strategy{
		ID:          strategyDomain.id,
		OwnerID:     strategyDomain.ownerID,
		Name:        strategyDomain.name,
		Description: strategyDomain.description,
		Script:      strategyDomain.script,
		ResultType:  string(strategyDomain.resultType.Value()),
		Parameters:  strategyDomain.parameters.ToEntities(),
	}
}

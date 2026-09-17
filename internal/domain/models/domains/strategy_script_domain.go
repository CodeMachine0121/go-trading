package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// strategyScriptNameMaxLength is how long a strategy script name may be, counted after the blanks
// around it are dropped. It is the single place this limit is written down.
const strategyScriptNameMaxLength = 128

// strategyScriptDescriptionMaxLength is how long a strategy script's description may be, counted
// after the blanks around it are dropped. It is generous because on the marketplace
// it is the only thing a reader has to go on — the script is never handed out — but
// it is bounded because a description is a paragraph, not a document.
const strategyScriptDescriptionMaxLength = 512

// StrategyScriptDomain holds one strategy script and guarantees its own invariants. An instance
// only exists when every rule passed, so there is no half-valid strategy script.
//
// A strategy script is an algorithm and nothing more: a name, a script and the kind of
// value that script produces. It holds no plan for feeding itself — how coarse the
// K candles are, how many of them, and up to when are settled by whoever runs it,
// so one algorithm can be run at any coarseness over any stretch of market instead
// of being saved once per way of looking at it.
//
// The result type already has a model that knows what it may be; this one borrows it
// rather than listing its values a second time. What it adds is the rest of the
// rules and a single kind of refusal, so that a caller never has to recognise a K
// candle's sentinel to find out its strategy script was rejected.
//
// It deliberately does not look at the script beyond "there is one". A script that
// cannot run is still worth saving: an algorithm usually takes several sittings to
// get right, and a strategy script that refused to be saved half-finished could not be
// picked up again tomorrow.
type StrategyScriptDomain struct {
	id          uint
	ownerID     uint
	name        string
	description string
	script      string
	resultType  IndicatorResultTypeDomain
	parameters  StrategyScriptParametersDomain
}

// NewStrategyScriptDomain validates the strategy script against every rule that applies to it.
// The rules are identical whether the strategy script is being created or rewritten,
// because both arrive here as the same shape.
func NewStrategyScriptDomain(writeDto dto.StrategyScriptWriteDto) (StrategyScriptDomain, error) {
	// A strategy script with nobody behind it is refused here rather than at the store,
	// because "every strategy script has an owner" is a rule about strategy scripts, not a
	// constraint that happens to exist on a column. Written here, it holds for
	// anything that ever builds one — including a future caller that does not go
	// through the same store.
	if writeDto.OwnerID == 0 {
		return StrategyScriptDomain{}, fmt.Errorf("%w: 策略腳本必須屬於一位使用者", ErrStrategyScriptValidation)
	}

	name := strings.TrimSpace(writeDto.Name)
	if name == "" {
		return StrategyScriptDomain{}, fmt.Errorf("%w: 必須給策略腳本取一個名稱", ErrStrategyScriptValidation)
	}

	if strings.ContainsRune(name, nulCharacter) {
		return StrategyScriptDomain{}, fmt.Errorf(
			"%w: 策略腳本名稱不得包含空字元（NUL）", ErrStrategyScriptValidation)
	}

	if len([]rune(name)) > strategyScriptNameMaxLength {
		return StrategyScriptDomain{}, fmt.Errorf(
			"%w: 策略腳本名稱長度上限為 %d 個字", ErrStrategyScriptValidation, strategyScriptNameMaxLength)
	}

	// A description is optional, so blanks and nothing are the same thing: somebody
	// who typed only spaces said nothing, and storing their spaces would make the
	// marketplace show an empty paragraph instead of no paragraph.
	description := strings.TrimSpace(writeDto.Description)
	if len([]rune(description)) > strategyScriptDescriptionMaxLength {
		return StrategyScriptDomain{}, fmt.Errorf(
			"%w: 策略腳本說明長度上限為 %d 個字", ErrStrategyScriptValidation, strategyScriptDescriptionMaxLength)
	}

	if strings.ContainsRune(description, nulCharacter) {
		return StrategyScriptDomain{}, fmt.Errorf(
			"%w: 策略腳本說明不得包含空字元（NUL）", ErrStrategyScriptValidation)
	}

	if strings.TrimSpace(writeDto.Script) == "" {
		return StrategyScriptDomain{}, fmt.Errorf("%w: 策略腳本必須帶一段指標算式", ErrStrategyScriptValidation)
	}

	if strings.ContainsRune(writeDto.Script, nulCharacter) {
		return StrategyScriptDomain{}, fmt.Errorf(
			"%w: 策略腳本算式不得包含空字元（NUL）", ErrStrategyScriptValidation)
	}

	resultType, resultTypeError := NewIndicatorResultTypeDomain(writeDto.ResultType)
	if resultTypeError != nil {
		return StrategyScriptDomain{}, fmt.Errorf("%w: %w", ErrStrategyScriptValidation, resultTypeError)
	}

	// The knobs already have a model that knows every rule about them; this one
	// borrows it rather than restating those rules a second time.
	parameters, parametersError := NewStrategyScriptParametersDomain(writeDto.Parameters)
	if parametersError != nil {
		return StrategyScriptDomain{}, fmt.Errorf("%w: %w", ErrStrategyScriptValidation, parametersError)
	}

	return StrategyScriptDomain{
		id:          writeDto.ID,
		ownerID:     writeDto.OwnerID,
		name:        name,
		description: description,
		script:      writeDto.Script,
		resultType:  resultType,
		parameters:  parameters,
	}, nil
}

// ResultType is the kind of value this strategy script produces, already read and accepted.
// It is handed out as the kind itself rather than as its spelling, so that a
// declaration is only ever interpreted once.
func (strategyScriptDomain StrategyScriptDomain) ResultType() IndicatorResultTypeDomain {
	return strategyScriptDomain.resultType
}

// ToEntity is the strategy script as it is stored. The times are left alone: when a
// strategy script was first saved and when it was last touched are recorded where the
// saving happens, not claimed here.
func (strategyScriptDomain StrategyScriptDomain) ToEntity() entities.StrategyScript {
	return entities.StrategyScript{
		ID:          strategyScriptDomain.id,
		OwnerID:     strategyScriptDomain.ownerID,
		Name:        strategyScriptDomain.name,
		Description: strategyScriptDomain.description,
		Script:      strategyScriptDomain.script,
		ResultType:  string(strategyScriptDomain.resultType.Value()),
		Parameters:  strategyScriptDomain.parameters.ToEntities(),
	}
}

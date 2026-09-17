package domains

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// ErrStrategyScriptParameterValidation is what every refusal about a strategy script's knobs
// carries, so a caller can tell "you described these wrongly" apart from anything
// else that can go wrong while saving or running.
var ErrStrategyScriptParameterValidation = errors.New("strategy script parameter validation failed")

// maximumStrategyScriptParameterNameLength bounds a name so it stays something a person
// reads on a screen. There is no count limit on the parameters themselves: names
// being unique and each value having a range is already enough to stop nonsense,
// and a limit on how many knobs an algorithm may have has no right answer.
const maximumStrategyScriptParameterNameLength = 64

// StrategyScriptParametersDomain is one strategy script's whole set of knobs, and every rule
// about them.
//
// It is a set rather than a parameter at a time because the rules that matter are
// rules about the set: names not repeating, and the largest look-back. Ask a single
// parameter either question and it cannot answer; leave the set to the caller and
// the caller ends up collecting and comparing by hand.
//
// Building one validates the whole set, so an instance existing means the set is
// usable and nobody downstream checks again.
type StrategyScriptParametersDomain struct {
	parameters []entities.StrategyScriptParameter
}

// NewStrategyScriptParametersDomain settles a whole set at once: names are trimmed and
// must be present and distinct, kinds must be one of the two, and a look-back count
// must be a whole number greater than zero.
//
// An empty set is valid and means an algorithm with no knobs, which is every
// algorithm written before knobs existed.
func NewStrategyScriptParametersDomain(
	declaredParameters []dto.StrategyScriptParameterWriteDto,
) (StrategyScriptParametersDomain, error) {
	settledParameters := make([]entities.StrategyScriptParameter, 0, len(declaredParameters))
	takenNames := make(map[string]struct{}, len(declaredParameters))

	for _, declaredParameter := range declaredParameters {
		settledParameter, settleError := settleStrategyScriptParameter(declaredParameter)
		if settleError != nil {
			return StrategyScriptParametersDomain{}, settleError
		}

		if _, isTaken := takenNames[settledParameter.Name]; isTaken {
			return StrategyScriptParametersDomain{}, fmt.Errorf(
				"%w: 參數名稱 %q 重複了，同一支策略腳本內不得重複",
				ErrStrategyScriptParameterValidation, settledParameter.Name)
		}
		takenNames[settledParameter.Name] = struct{}{}

		settledParameters = append(settledParameters, settledParameter)
	}

	return StrategyScriptParametersDomain{parameters: settledParameters}, nil
}

// Applying settles what this run's knobs are worth: every supplied name must have
// been declared, whatever was not supplied keeps its declared default, and every
// resulting value must still be within its kind's range.
//
// It is the only way to apply values, and it answers all four of those at once, so
// no caller has to sequence them and none can be skipped.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) Applying(
	suppliedValues []dto.StrategyScriptParameterValueDto,
) (StrategyScriptParametersDomain, error) {
	suppliedByName := make(map[string]float64, len(suppliedValues))
	for _, suppliedValue := range suppliedValues {
		suppliedName := strings.TrimSpace(suppliedValue.Name)
		if !strategyScriptParametersDomain.declares(suppliedName) {
			return StrategyScriptParametersDomain{}, fmt.Errorf(
				"%w: %q 不是這支策略腳本的參數",
				ErrStrategyScriptParameterValidation, suppliedValue.Name)
		}
		suppliedByName[suppliedName] = suppliedValue.Value
	}

	appliedParameters := make([]entities.StrategyScriptParameter, 0, len(strategyScriptParametersDomain.parameters))
	for _, declaredParameter := range strategyScriptParametersDomain.parameters {
		appliedParameter := declaredParameter
		if suppliedValue, isSupplied := suppliedByName[declaredParameter.Name]; isSupplied {
			appliedParameter.DefaultValue = suppliedValue
		}

		if valueError := validateStrategyScriptParameterValue(appliedParameter); valueError != nil {
			return StrategyScriptParametersDomain{}, valueError
		}

		appliedParameters = append(appliedParameters, settleStrategyScriptParameterValue(appliedParameter))
	}

	return StrategyScriptParametersDomain{parameters: appliedParameters}, nil
}

// MaximumLookbackCount is how far back the hungriest of these knobs reaches. Zero
// means nothing reaches back, which is what an algorithm with no look-back knobs
// wants — and it makes the count derivation one expression rather than two branches.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) MaximumLookbackCount() int {
	maximumLookbackCount := 0
	for _, parameter := range strategyScriptParametersDomain.parameters {
		if !parameter.IsLookbackCount() {
			continue
		}
		maximumLookbackCount = max(maximumLookbackCount, int(parameter.DefaultValue))
	}

	return maximumLookbackCount
}

// LookbackCountOf hands a script the whole number behind a name, saying whether the
// name was declared at all — that second answer is what keeps a mistyped name from
// being reported as a broken algorithm.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) LookbackCountOf(name string) (int, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return int(parameter.DefaultValue), true
}

// BooleanOf hands a script the yes-or-no behind a name, saying whether the name was
// declared at all. Zero is no and anything else is yes — but a declared boolean has
// already been settled to exactly zero or one, so this reads what was settled.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) BooleanOf(name string) (bool, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return false, false
	}

	return parameter.IsTrue(), true
}

// NumberOf hands a script the number behind a name, saying whether the name was
// declared at all.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) NumberOf(name string) (float64, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return parameter.DefaultValue, true
}

// ToEntities hands the settled set back for storing.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) ToEntities() []entities.StrategyScriptParameter {
	storedParameters := make([]entities.StrategyScriptParameter, len(strategyScriptParametersDomain.parameters))
	copy(storedParameters, strategyScriptParametersDomain.parameters)

	return storedParameters
}

// ToDtos hands the settled set outwards.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) ToDtos() []dto.StrategyScriptParameterDto {
	parameterDtos := make([]dto.StrategyScriptParameterDto, 0, len(strategyScriptParametersDomain.parameters))
	for _, parameter := range strategyScriptParametersDomain.parameters {
		parameterDtos = append(parameterDtos, parameter.ToDto())
	}

	return parameterDtos
}

func (strategyScriptParametersDomain StrategyScriptParametersDomain) find(
	name string,
) (entities.StrategyScriptParameter, bool) {
	for _, parameter := range strategyScriptParametersDomain.parameters {
		if parameter.Name == name {
			return parameter, true
		}
	}

	return entities.StrategyScriptParameter{}, false
}

func (strategyScriptParametersDomain StrategyScriptParametersDomain) declares(name string) bool {
	_, isDeclared := strategyScriptParametersDomain.find(name)

	return isDeclared
}

// settleStrategyScriptParameter normalizes one declaration and refuses what cannot be one.
func settleStrategyScriptParameter(
	declaredParameter dto.StrategyScriptParameterWriteDto,
) (entities.StrategyScriptParameter, error) {
	settledName := strings.TrimSpace(declaredParameter.Name)
	if settledName == "" {
		return entities.StrategyScriptParameter{}, fmt.Errorf(
			"%w: 參數名稱不得為空白", ErrStrategyScriptParameterValidation)
	}
	if len([]rune(settledName)) > maximumStrategyScriptParameterNameLength {
		return entities.StrategyScriptParameter{}, fmt.Errorf(
			"%w: 參數名稱 %q 超過 %d 個字",
			ErrStrategyScriptParameterValidation, settledName, maximumStrategyScriptParameterNameLength)
	}

	kindDomain, kindError := NewStrategyScriptParameterKindDomain(declaredParameter.Kind)
	if kindError != nil {
		return entities.StrategyScriptParameter{}, fmt.Errorf(
			"%w: %v", ErrStrategyScriptParameterValidation, kindError)
	}

	settledParameter := entities.StrategyScriptParameter{
		Name:         settledName,
		Kind:         string(kindDomain.Value()),
		DefaultValue: declaredParameter.DefaultValue,
	}

	settledParameter = settleStrategyScriptParameterValue(settledParameter)
	if valueError := validateStrategyScriptParameterValue(settledParameter); valueError != nil {
		return entities.StrategyScriptParameter{}, valueError
	}

	return settledParameter, nil
}

// validateStrategyScriptParameterValue judges one value against its own kind. A look-back
// count has to be a whole number greater than zero — it is going to be used to count
// candles, and half a candle is not a thing. A number is not judged at all: the
// system reads no meaning into it, so it has no grounds to refuse one.
func validateStrategyScriptParameterValue(parameter entities.StrategyScriptParameter) error {
	if !parameter.IsLookbackCount() {
		return nil
	}

	if parameter.DefaultValue < 1 || parameter.DefaultValue != math.Trunc(parameter.DefaultValue) {
		return fmt.Errorf(
			"%w: 參數 %q 是回看根數，必須是大於零的整數，收到 %v",
			ErrStrategyScriptParameterValidation, parameter.Name, parameter.DefaultValue)
	}

	return nil
}

// settleStrategyScriptParameterValue pins a yes-or-no to exactly zero or one.
//
// It is settled rather than refused because there is nothing to refuse: every number
// is either zero or not. Settling it means what is stored says plainly which of the
// two it is, instead of leaving a 0.7 for whoever reads it next to interpret — and
// it makes a value that went in as 2 come back out as 1, rather than as 2.
func settleStrategyScriptParameterValue(
	parameter entities.StrategyScriptParameter,
) entities.StrategyScriptParameter {
	if !parameter.IsBoolean() {
		return parameter
	}

	settledParameter := parameter
	settledParameter.DefaultValue = 0
	if parameter.IsTrue() {
		settledParameter.DefaultValue = 1
	}

	return settledParameter
}

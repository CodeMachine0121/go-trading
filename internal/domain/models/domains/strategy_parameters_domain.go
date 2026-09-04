package domains

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// ErrStrategyParameterValidation is what every refusal about a strategy's knobs
// carries, so a caller can tell "you described these wrongly" apart from anything
// else that can go wrong while saving or running.
var ErrStrategyParameterValidation = errors.New("strategy parameter validation failed")

// maximumStrategyParameterNameLength bounds a name so it stays something a person
// reads on a screen. There is no count limit on the parameters themselves: names
// being unique and each value having a range is already enough to stop nonsense,
// and a limit on how many knobs an algorithm may have has no right answer.
const maximumStrategyParameterNameLength = 64

// StrategyParametersDomain is one strategy's whole set of knobs, and every rule
// about them.
//
// It is a set rather than a parameter at a time because the rules that matter are
// rules about the set: names not repeating, and the largest look-back. Ask a single
// parameter either question and it cannot answer; leave the set to the caller and
// the caller ends up collecting and comparing by hand.
//
// Building one validates the whole set, so an instance existing means the set is
// usable and nobody downstream checks again.
type StrategyParametersDomain struct {
	parameters []entities.StrategyParameter
}

// NewStrategyParametersDomain settles a whole set at once: names are trimmed and
// must be present and distinct, kinds must be one of the two, and a look-back count
// must be a whole number greater than zero.
//
// An empty set is valid and means an algorithm with no knobs, which is every
// algorithm written before knobs existed.
func NewStrategyParametersDomain(
	declaredParameters []dto.StrategyParameterWriteDto,
) (StrategyParametersDomain, error) {
	settledParameters := make([]entities.StrategyParameter, 0, len(declaredParameters))
	takenNames := make(map[string]struct{}, len(declaredParameters))

	for _, declaredParameter := range declaredParameters {
		settledParameter, settleError := settleStrategyParameter(declaredParameter)
		if settleError != nil {
			return StrategyParametersDomain{}, settleError
		}

		if _, isTaken := takenNames[settledParameter.Name]; isTaken {
			return StrategyParametersDomain{}, fmt.Errorf(
				"%w: 參數名稱 %q 重複了，同一支策略內不得重複",
				ErrStrategyParameterValidation, settledParameter.Name)
		}
		takenNames[settledParameter.Name] = struct{}{}

		settledParameters = append(settledParameters, settledParameter)
	}

	return StrategyParametersDomain{parameters: settledParameters}, nil
}

// Applying settles what this run's knobs are worth: every supplied name must have
// been declared, whatever was not supplied keeps its declared default, and every
// resulting value must still be within its kind's range.
//
// It is the only way to apply values, and it answers all four of those at once, so
// no caller has to sequence them and none can be skipped.
func (strategyParametersDomain StrategyParametersDomain) Applying(
	suppliedValues []dto.StrategyParameterValueDto,
) (StrategyParametersDomain, error) {
	suppliedByName := make(map[string]float64, len(suppliedValues))
	for _, suppliedValue := range suppliedValues {
		suppliedName := strings.TrimSpace(suppliedValue.Name)
		if !strategyParametersDomain.declares(suppliedName) {
			return StrategyParametersDomain{}, fmt.Errorf(
				"%w: %q 不是這支策略的參數",
				ErrStrategyParameterValidation, suppliedValue.Name)
		}
		suppliedByName[suppliedName] = suppliedValue.Value
	}

	appliedParameters := make([]entities.StrategyParameter, 0, len(strategyParametersDomain.parameters))
	for _, declaredParameter := range strategyParametersDomain.parameters {
		appliedParameter := declaredParameter
		if suppliedValue, isSupplied := suppliedByName[declaredParameter.Name]; isSupplied {
			appliedParameter.DefaultValue = suppliedValue
		}

		if valueError := validateStrategyParameterValue(appliedParameter); valueError != nil {
			return StrategyParametersDomain{}, valueError
		}

		appliedParameters = append(appliedParameters, appliedParameter)
	}

	return StrategyParametersDomain{parameters: appliedParameters}, nil
}

// MaximumLookbackCount is how far back the hungriest of these knobs reaches. Zero
// means nothing reaches back, which is what an algorithm with no look-back knobs
// wants — and it makes the count derivation one expression rather than two branches.
func (strategyParametersDomain StrategyParametersDomain) MaximumLookbackCount() int {
	maximumLookbackCount := 0
	for _, parameter := range strategyParametersDomain.parameters {
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
func (strategyParametersDomain StrategyParametersDomain) LookbackCountOf(name string) (int, bool) {
	parameter, isDeclared := strategyParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return int(parameter.DefaultValue), true
}

// NumberOf hands a script the number behind a name, saying whether the name was
// declared at all.
func (strategyParametersDomain StrategyParametersDomain) NumberOf(name string) (float64, bool) {
	parameter, isDeclared := strategyParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return parameter.DefaultValue, true
}

// ToEntities hands the settled set back for storing.
func (strategyParametersDomain StrategyParametersDomain) ToEntities() []entities.StrategyParameter {
	storedParameters := make([]entities.StrategyParameter, len(strategyParametersDomain.parameters))
	copy(storedParameters, strategyParametersDomain.parameters)

	return storedParameters
}

// ToDtos hands the settled set outwards.
func (strategyParametersDomain StrategyParametersDomain) ToDtos() []dto.StrategyParameterDto {
	parameterDtos := make([]dto.StrategyParameterDto, 0, len(strategyParametersDomain.parameters))
	for _, parameter := range strategyParametersDomain.parameters {
		parameterDtos = append(parameterDtos, parameter.ToDto())
	}

	return parameterDtos
}

func (strategyParametersDomain StrategyParametersDomain) find(
	name string,
) (entities.StrategyParameter, bool) {
	for _, parameter := range strategyParametersDomain.parameters {
		if parameter.Name == name {
			return parameter, true
		}
	}

	return entities.StrategyParameter{}, false
}

func (strategyParametersDomain StrategyParametersDomain) declares(name string) bool {
	_, isDeclared := strategyParametersDomain.find(name)

	return isDeclared
}

// settleStrategyParameter normalizes one declaration and refuses what cannot be one.
func settleStrategyParameter(
	declaredParameter dto.StrategyParameterWriteDto,
) (entities.StrategyParameter, error) {
	settledName := strings.TrimSpace(declaredParameter.Name)
	if settledName == "" {
		return entities.StrategyParameter{}, fmt.Errorf(
			"%w: 參數名稱不得為空白", ErrStrategyParameterValidation)
	}
	if len([]rune(settledName)) > maximumStrategyParameterNameLength {
		return entities.StrategyParameter{}, fmt.Errorf(
			"%w: 參數名稱 %q 超過 %d 個字",
			ErrStrategyParameterValidation, settledName, maximumStrategyParameterNameLength)
	}

	kindDomain, kindError := NewStrategyParameterKindDomain(declaredParameter.Kind)
	if kindError != nil {
		return entities.StrategyParameter{}, fmt.Errorf(
			"%w: %v", ErrStrategyParameterValidation, kindError)
	}

	settledParameter := entities.StrategyParameter{
		Name:         settledName,
		Kind:         string(kindDomain.Value()),
		DefaultValue: declaredParameter.DefaultValue,
	}

	if valueError := validateStrategyParameterValue(settledParameter); valueError != nil {
		return entities.StrategyParameter{}, valueError
	}

	return settledParameter, nil
}

// validateStrategyParameterValue judges one value against its own kind. A look-back
// count has to be a whole number greater than zero — it is going to be used to count
// candles, and half a candle is not a thing. A number is not judged at all: the
// system reads no meaning into it, so it has no grounds to refuse one.
func validateStrategyParameterValue(parameter entities.StrategyParameter) error {
	if !parameter.IsLookbackCount() {
		return nil
	}

	if parameter.DefaultValue < 1 || parameter.DefaultValue != math.Trunc(parameter.DefaultValue) {
		return fmt.Errorf(
			"%w: 參數 %q 是回看根數，必須是大於零的整數，收到 %v",
			ErrStrategyParameterValidation, parameter.Name, parameter.DefaultValue)
	}

	return nil
}

package domains

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

var ErrStrategyScriptParameterValidation = errors.New("strategy script parameter validation failed")

// maximumStrategyScriptParameterNameLength keeps names screen-readable; the number of parameters is deliberately unbounded.
const maximumStrategyScriptParameterNameLength = 64

// StrategyScriptParametersDomain is a validated whole set of parameters, since uniqueness and the maximum look-back are set-level rules.
type StrategyScriptParametersDomain struct {
	parameters []entities.StrategyScriptParameter
}

// NewStrategyScriptParametersDomain validates the set; an empty set is valid.
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

// Applying rejects undeclared names, keeps defaults for unsupplied values and re-validates every resulting value.
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

// MaximumLookbackCount is zero when no parameter is a look-back count.
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

// LookbackCountOf's second result distinguishes an undeclared name from a broken algorithm.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) LookbackCountOf(name string) (int, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return int(parameter.DefaultValue), true
}

// BooleanOf reads the value already settled to exactly zero or one.
func (strategyScriptParametersDomain StrategyScriptParametersDomain) BooleanOf(name string) (bool, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return false, false
	}

	return parameter.IsTrue(), true
}

func (strategyScriptParametersDomain StrategyScriptParametersDomain) NumberOf(name string) (float64, bool) {
	parameter, isDeclared := strategyScriptParametersDomain.find(name)
	if !isDeclared {
		return 0, false
	}

	return parameter.DefaultValue, true
}

func (strategyScriptParametersDomain StrategyScriptParametersDomain) ToEntities() []entities.StrategyScriptParameter {
	storedParameters := make([]entities.StrategyScriptParameter, len(strategyScriptParametersDomain.parameters))
	copy(storedParameters, strategyScriptParametersDomain.parameters)

	return storedParameters
}

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

// validateStrategyScriptParameterValue requires look-back counts to be positive integers; plain numbers are never judged.
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

// settleStrategyScriptParameterValue normalises booleans to exactly zero or one instead of refusing them.
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

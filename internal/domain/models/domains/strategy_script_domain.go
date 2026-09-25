package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// strategyScriptNameMaxLength is counted in runes after trimming.
const strategyScriptNameMaxLength = 128

// strategyScriptDescriptionMaxLength is generous because the description is all a marketplace reader sees (the script is never shared).
const strategyScriptDescriptionMaxLength = 512

// StrategyScriptDomain is a validated strategy script: an algorithm with no feeding plan (interval, count and range are the runner's), and the script body is only checked for presence so half-finished work can still be saved.
type StrategyScriptDomain struct {
	id          uint
	ownerID     uint
	name        string
	description string
	script      string
	resultType  IndicatorResultTypeDomain
	// marketDataKind arrives already reconciled on rewrite (see MarketDataKindDomain.Retaining).
	marketDataKind MarketDataKindDomain
	parameters     StrategyScriptParametersDomain
}

func NewStrategyScriptDomain(writeDto dto.StrategyScriptWriteDto) (StrategyScriptDomain, error) {
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

	// Blank-only descriptions become empty so the marketplace shows no paragraph rather than an empty one.
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

	marketDataKind, marketDataKindError := NewMarketDataKindDomain(writeDto.MarketDataKind)
	if marketDataKindError != nil {
		return StrategyScriptDomain{}, fmt.Errorf("%w: %w", ErrStrategyScriptValidation, marketDataKindError)
	}

	parameters, parametersError := NewStrategyScriptParametersDomain(writeDto.Parameters)
	if parametersError != nil {
		return StrategyScriptDomain{}, fmt.Errorf("%w: %w", ErrStrategyScriptValidation, parametersError)
	}

	return StrategyScriptDomain{
		id:             writeDto.ID,
		ownerID:        writeDto.OwnerID,
		name:           name,
		description:    description,
		script:         writeDto.Script,
		resultType:     resultType,
		marketDataKind: marketDataKind,
		parameters:     parameters,
	}, nil
}

// ResultType is handed out as the parsed kind so the declaration is interpreted only once.
func (strategyScriptDomain StrategyScriptDomain) ResultType() IndicatorResultTypeDomain {
	return strategyScriptDomain.resultType
}

// ToEntity leaves timestamps to the store.
func (strategyScriptDomain StrategyScriptDomain) ToEntity() entities.StrategyScript {
	return entities.StrategyScript{
		ID:             strategyScriptDomain.id,
		OwnerID:        strategyScriptDomain.ownerID,
		Name:           strategyScriptDomain.name,
		Description:    strategyScriptDomain.description,
		Script:         strategyScriptDomain.script,
		ResultType:     string(strategyScriptDomain.resultType.Value()),
		MarketDataKind: string(strategyScriptDomain.marketDataKind.Value()),
		Parameters:     strategyScriptDomain.parameters.ToEntities(),
	}
}

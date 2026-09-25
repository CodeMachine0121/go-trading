package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyScriptRequest serves both save and full rewrite; the ID comes from the path, and candle interval and count belong to a calculation, not the script.
type StrategyScriptRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Script      string `json:"script"`
	ResultType  string `json:"resultType"`
	// MarketDataKind is kCandle (default on create) or contractKCandle; omitted on rewrite keeps the existing kind.
	MarketDataKind string                           `json:"marketDataKind"`
	Parameters     []StrategyScriptParameterRequest `json:"parameters"`
}

// ToWriteDto takes the ID (zero means new) and the owner from the arguments, since the owner must come from the token, never the body.
func (strategyScriptRequest StrategyScriptRequest) ToWriteDto(id uint, ownerID uint) dto.StrategyScriptWriteDto {
	return dto.StrategyScriptWriteDto{
		ID:             id,
		OwnerID:        ownerID,
		Name:           strategyScriptRequest.Name,
		Description:    strategyScriptRequest.Description,
		Script:         strategyScriptRequest.Script,
		ResultType:     strategyScriptRequest.ResultType,
		MarketDataKind: strategyScriptRequest.MarketDataKind,
		Parameters:     strategyScriptRequest.parameterWriteDtos(),
	}
}

// parameterWriteDtos always returns a list, empty rather than nil when no parameters are declared.
func (strategyScriptRequest StrategyScriptRequest) parameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(strategyScriptRequest.Parameters))
	for _, parameterRequest := range strategyScriptRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

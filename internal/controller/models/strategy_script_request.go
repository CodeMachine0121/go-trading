package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyScriptRequest is the body a caller sends to save or rewrite a strategy script.
//
// One shape serves both, because a rewrite replaces everything a strategy script
// remembers — there is no field a caller may set on the way in and not on the way
// back. Which strategy script is meant comes from the path, never from the body.
//
// How coarse the K candles are and how many of them are absent on purpose: they
// belong to a calculation, not to a strategy script. A caller that sends them anyway is
// simply sending fields nothing binds to.
type StrategyScriptRequest struct {
	Name string `json:"name"`
	// Description is what this strategy script is for, in the owner's words. It may be
	// left out; on the marketplace it is then the only thing missing from the only
	// thing a reader has.
	Description string `json:"description"`
	Script      string `json:"script"`
	ResultType  string `json:"resultType"`
	// Parameters are the algorithm's own knobs. Leaving them out declares an
	// algorithm with no knobs, which is what every algorithm was before knobs.
	Parameters []StrategyScriptParameterRequest `json:"parameters"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking the
// identity and the owner from the arguments so the caller of this method decides
// both. A zero identifier means a strategy script that does not exist yet.
//
// The owner is deliberately not a field on the request. A body that could name its
// own owner is a body that could claim somebody else's — who is asking comes from
// the proof of identity on the request, never from what the request says about
// itself.
func (strategyScriptRequest StrategyScriptRequest) ToWriteDto(id uint, ownerID uint) dto.StrategyScriptWriteDto {
	return dto.StrategyScriptWriteDto{
		ID:          id,
		OwnerID:     ownerID,
		Name:        strategyScriptRequest.Name,
		Description: strategyScriptRequest.Description,
		Script:      strategyScriptRequest.Script,
		ResultType:  strategyScriptRequest.ResultType,
		Parameters:  strategyScriptRequest.parameterWriteDtos(),
	}
}

// parameterWriteDtos hands the declarations on untouched, always as a list rather
// than sometimes nothing: declaring no knobs is an empty list, not an absence.
func (strategyScriptRequest StrategyScriptRequest) parameterWriteDtos() []dto.StrategyScriptParameterWriteDto {
	parameterWriteDtos := make([]dto.StrategyScriptParameterWriteDto, 0, len(strategyScriptRequest.Parameters))
	for _, parameterRequest := range strategyScriptRequest.Parameters {
		parameterWriteDtos = append(parameterWriteDtos, parameterRequest.ToWriteDto())
	}

	return parameterWriteDtos
}

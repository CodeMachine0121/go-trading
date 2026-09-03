package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyRequest is the body a caller sends to save or rewrite a strategy.
//
// One shape serves both, because a rewrite replaces everything a strategy
// remembers — there is no field a caller may set on the way in and not on the way
// back. Which strategy is meant comes from the path, never from the body.
//
// How coarse the K candles are and how many of them are absent on purpose: they
// belong to a calculation, not to a strategy. A caller that sends them anyway is
// simply sending fields nothing binds to.
type StrategyRequest struct {
	Name                string `json:"name"`
	Script              string `json:"script"`
	ResultType          string `json:"resultType"`
}

// ToWriteDto turns the request into the shape the domain accepts, taking the
// identity from the argument so the caller of this method decides what is named.
// A zero identifier means a strategy that does not exist yet.
func (strategyRequest StrategyRequest) ToWriteDto(id uint) dto.StrategyWriteDto {
	return dto.StrategyWriteDto{
		ID:                  id,
		Name:                strategyRequest.Name,
		Script:              strategyRequest.Script,
		ResultType:          strategyRequest.ResultType,
	}
}

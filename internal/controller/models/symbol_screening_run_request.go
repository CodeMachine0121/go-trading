package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// SymbolScreeningRunRequest takes the symbols as pasted, separated by commas.
type SymbolScreeningRunRequest struct {
	ProfileID uint   `json:"profileId"`
	Symbols   string `json:"symbols"`
}

func (request SymbolScreeningRunRequest) ToSubmissionDto() dto.SymbolScreeningRunSubmissionDto {
	return dto.SymbolScreeningRunSubmissionDto{ProfileID: request.ProfileID, SymbolList: request.Symbols}
}

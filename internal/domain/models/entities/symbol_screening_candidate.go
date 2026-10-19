package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type SymbolScreeningCandidate struct {
	ID       uint                           `gorm:"primaryKey"`
	RunID    uint                           `gorm:"not null;index:idx_symbol_screening_candidates_run_id_position,priority:1"`
	Position int                            `gorm:"not null;index:idx_symbol_screening_candidates_run_id_position,priority:2"`
	Symbol   string                         `gorm:"size:64;not null"`
	Status   string                         `gorm:"size:16;not null"`
	Stage    string                         `gorm:"size:32;not null;default:''"`
	Reason   string                         `gorm:"type:text;not null;default:''"`
	Findings dto.SymbolScreeningFindingsDto `gorm:"serializer:json;type:jsonb;not null"`
	// RecommendationRank is nil until the run ranks its candidates at the end.
	RecommendationRank *int
}

func (symbolScreeningCandidate SymbolScreeningCandidate) TableName() string {
	return "SymbolScreeningCandidates"
}

func (symbolScreeningCandidate SymbolScreeningCandidate) ToDto() dto.SymbolScreeningCandidateDto {
	return dto.SymbolScreeningCandidateDto{
		Symbol:             symbolScreeningCandidate.Symbol,
		Position:           symbolScreeningCandidate.Position,
		Status:             symbolScreeningCandidate.Status,
		Stage:              symbolScreeningCandidate.Stage,
		Reason:             symbolScreeningCandidate.Reason,
		Findings:           symbolScreeningCandidate.Findings,
		RecommendationRank: symbolScreeningCandidate.RecommendationRank,
	}
}

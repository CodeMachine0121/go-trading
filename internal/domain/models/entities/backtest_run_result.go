package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// BacktestRunResult keeps the full report apart from the run, so listing runs never carries equity curves around.
type BacktestRunResult struct {
	BacktestRunID uint `gorm:"primaryKey"`
	// Exactly one of the two is set, by the run's kind.
	SpotResult     *dto.BacktestResultDto         `gorm:"serializer:json;type:jsonb"`
	ContractResult *dto.ContractBacktestResultDto `gorm:"serializer:json;type:jsonb"`
}

func (backtestRunResult BacktestRunResult) TableName() string {
	return "BacktestRunResults"
}

func (backtestRunResult BacktestRunResult) ToDto() dto.BacktestRunResultDto {
	return dto.BacktestRunResultDto{
		Spot:     backtestRunResult.SpotResult,
		Contract: backtestRunResult.ContractResult,
	}
}

package assistantqueries

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradingStrategyRevisionApplier reads a rewrite in the shape update_trading_strategy takes and carries it out
// through the same rewrite a person makes, running-bot refusal included.
type TradingStrategyRevisionApplier struct {
	tradingStrategyApplication *application.TradingStrategyApplication
}

func NewTradingStrategyRevisionApplier(
	tradingStrategyApplication *application.TradingStrategyApplication,
) *TradingStrategyRevisionApplier {
	return &TradingStrategyRevisionApplier{tradingStrategyApplication: tradingStrategyApplication}
}

func (tradingStrategyRevisionApplier *TradingStrategyRevisionApplier) SubjectKind() vo.AssistantRevisionSubjectKindVo {
	return vo.AssistantRevisionSubjectTradingStrategy
}

func (tradingStrategyRevisionApplier *TradingStrategyRevisionApplier) Inspect(
	executionContext context.Context, viewerID uint, content string,
) (dto.RewriteTargetDto, error) {
	writeDto, readError := tradingStrategyRevisionApplier.writeDtoOf(content)
	if readError != nil {
		return dto.RewriteTargetDto{}, readError
	}

	return tradingStrategyRevisionApplier.tradingStrategyApplication.InspectTradingStrategyRewrite(
		executionContext, viewerID, writeDto)
}

func (tradingStrategyRevisionApplier *TradingStrategyRevisionApplier) Apply(
	executionContext context.Context, viewerID uint, content string,
) (string, error) {
	writeDto, readError := tradingStrategyRevisionApplier.writeDtoOf(content)
	if readError != nil {
		return "", readError
	}

	tradingStrategyDto, updateError := tradingStrategyRevisionApplier.tradingStrategyApplication.
		UpdateTradingStrategy(executionContext, viewerID, writeDto)
	if updateError != nil {
		return "", updateError
	}

	return renderedTradingStrategy(tradingStrategyDto)
}

func (tradingStrategyRevisionApplier *TradingStrategyRevisionApplier) writeDtoOf(
	content string,
) (dto.TradingStrategyWriteDto, error) {
	// Unknown fields are refused rather than dropped, so what the owner reviews is exactly what gets written.
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	writeArguments := tradingStrategyWriteAssistantArguments{}
	if unmarshalError := decoder.Decode(&writeArguments); unmarshalError != nil {
		return dto.TradingStrategyWriteDto{}, fmt.Errorf(
			"%w: 參數不是合法的 JSON: %s", domains.ErrAssistantQueryArgument, unmarshalError)
	}
	// Anything after the arguments would be shown to the owner yet never written, and would break reading them back.
	if decoder.More() {
		return dto.TradingStrategyWriteDto{}, fmt.Errorf(
			"%w: 參數只能是一個 JSON 物件，後面不得再有其他內容", domains.ErrAssistantQueryArgument)
	}

	return writeArguments.ToWriteDto(writeArguments.TradingStrategyID), nil
}

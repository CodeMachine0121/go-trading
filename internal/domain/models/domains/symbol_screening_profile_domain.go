package domains

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	symbolScreeningProfileNameMaximumLength = 128
	symbolScreeningMaximumBaselineSymbols   = 10
)

var symbolScreeningPercentageCeiling = decimal.NewFromInt(100)

var symbolScreeningIntervalSteps = map[string]time.Duration{
	string(vo.AggregationIntervalFourHours): 4 * time.Hour,
	string(vo.AggregationIntervalOneDay):    24 * time.Hour,
}

// SymbolScreeningProfileDomain fills unset figures with the defaults and keeps every figure inside the range screening can judge.
type SymbolScreeningProfileDomain struct {
	profile entities.SymbolScreeningProfile
}

func NewSymbolScreeningProfileDomain(
	ownerID uint, writeDto dto.SymbolScreeningProfileWriteDto,
) (SymbolScreeningProfileDomain, error) {
	name := strings.TrimSpace(writeDto.Name)
	if name == "" || utf8.RuneCountInString(name) > symbolScreeningProfileNameMaximumLength {
		return SymbolScreeningProfileDomain{}, fmt.Errorf(
			"%w: 名稱要有 1 到 %d 個字", ErrSymbolScreeningValidation, symbolScreeningProfileNameMaximumLength)
	}
	if writeDto.StrategyScriptID == 0 {
		return SymbolScreeningProfileDomain{}, fmt.Errorf("%w: 請指定策略腳本", ErrSymbolScreeningValidation)
	}
	if _, isScreenable := symbolScreeningIntervalSteps[writeDto.AggregationInterval]; !isScreenable {
		return SymbolScreeningProfileDomain{}, fmt.Errorf(
			"%w: 看盤間隔只能是每 4 小時或每天", ErrSymbolScreeningValidation)
	}

	tradingModeDeclaration := writeDto.TradingMode
	if strings.TrimSpace(tradingModeDeclaration) == "" {
		tradingModeDeclaration = string(vo.ContractTradingModeLongOnly)
	}
	tradingMode, tradingModeError := NewContractTradingModeDomain(tradingModeDeclaration)
	if tradingModeError != nil {
		return SymbolScreeningProfileDomain{}, fmt.Errorf("%w: %w", ErrSymbolScreeningValidation, tradingModeError)
	}

	figures := []struct {
		supplied *int
		fallback int
		minimum  int
		maximum  int
		name     string
	}{
		{writeDto.HistoryDays, 730, 180, 1825, "歷史天數"},
		{writeDto.SegmentCount, 3, 2, 6, "區段數"},
		{writeDto.MinimumTradeCount, 6, 1, 200, "最少交易筆數"},
	}
	settledFigures := make([]int, 0, len(figures))
	for _, figure := range figures {
		value := figure.fallback
		if figure.supplied != nil {
			value = *figure.supplied
		}
		if value < figure.minimum || value > figure.maximum {
			return SymbolScreeningProfileDomain{}, fmt.Errorf(
				"%w: %s要介於 %d 到 %d", ErrSymbolScreeningValidation, figure.name, figure.minimum, figure.maximum)
		}
		settledFigures = append(settledFigures, value)
	}

	amounts := []struct {
		supplied *decimal.Decimal
		fallback decimal.Decimal
		minimum  decimal.Decimal
		maximum  *decimal.Decimal
		name     string
	}{
		{writeDto.Leverage, decimal.NewFromInt(1), decimal.NewFromInt(1), nil, "槓桿倍數"},
		{writeDto.EntryCostPercentage, decimal.RequireFromString("0.05"), decimal.Zero, &symbolScreeningPercentageCeiling, "手續費百分比"},
		{writeDto.SlippagePercentage, decimal.RequireFromString("0.05"), decimal.Zero, &symbolScreeningPercentageCeiling, "滑價百分比"},
		{writeDto.MinimumAverageDailyQuoteVolume, decimal.NewFromInt(20_000_000), decimal.Zero, nil, "最低日均成交額"},
	}
	settledAmounts := make([]decimal.Decimal, 0, len(amounts))
	for _, amount := range amounts {
		value := amount.fallback
		if amount.supplied != nil {
			value = *amount.supplied
		}
		if value.LessThan(amount.minimum) || (amount.maximum != nil && value.GreaterThan(*amount.maximum)) {
			return SymbolScreeningProfileDomain{}, fmt.Errorf(
				"%w: %s超出可接受的範圍", ErrSymbolScreeningValidation, amount.name)
		}
		settledAmounts = append(settledAmounts, value)
	}

	baselineSymbols, baselineError := NewSymbolScreeningSymbolListDomain(
		strings.Join(writeDto.BaselineSymbols, ","), symbolScreeningMaximumBaselineSymbols, true)
	if baselineError != nil {
		return SymbolScreeningProfileDomain{}, baselineError
	}

	parameterValues := writeDto.ParameterValues
	if parameterValues == nil {
		parameterValues = []dto.StrategyScriptParameterValueDto{}
	}

	return SymbolScreeningProfileDomain{profile: entities.SymbolScreeningProfile{
		OwnerID:                        ownerID,
		Name:                           name,
		StrategyScriptID:               writeDto.StrategyScriptID,
		ParameterValues:                parameterValues,
		AggregationInterval:            writeDto.AggregationInterval,
		TradingMode:                    string(tradingMode.Value()),
		Leverage:                       settledAmounts[0],
		EntryCostPercentage:            settledAmounts[1],
		SlippagePercentage:             settledAmounts[2],
		HistoryDays:                    settledFigures[0],
		SegmentCount:                   settledFigures[1],
		MinimumTradeCount:              settledFigures[2],
		MinimumAverageDailyQuoteVolume: settledAmounts[3],
		BaselineSymbols:                baselineSymbols.Symbols(),
	}}, nil
}

// AcceptScript refuses a script screening cannot replay: spot, non-signal, order-flow, or one whose declared knobs reject the profile's values.
func (profileDomain SymbolScreeningProfileDomain) AcceptScript(runnableScript dto.RunnableStrategyScriptDto) error {
	if runnableScript.MarketDataKind != string(vo.MarketDataKindContractKCandle) {
		return fmt.Errorf("%w: 策略腳本必須吃合約行情", ErrSymbolScreeningValidation)
	}
	if runnableScript.ResultType != string(vo.IndicatorResultTypeSignal) {
		return fmt.Errorf("%w: 策略腳本必須產出買賣信號", ErrSymbolScreeningValidation)
	}
	parameters, parametersError := NewStrategyScriptParametersDomain(runnableScript.Parameters)
	if parametersError != nil {
		return fmt.Errorf("%w: %w", ErrSymbolScreeningValidation, parametersError)
	}
	if _, declaresOrderFlow := parameters.OrderFlowPriceStep(); declaresOrderFlow {
		return fmt.Errorf("%w: 篩選用行情沒有逐筆成交明細，不能用需要訂單流的策略腳本", ErrSymbolScreeningValidation)
	}
	if _, applyError := parameters.Applying(profileDomain.profile.ParameterValues); applyError != nil {
		return fmt.Errorf("%w: %w", ErrSymbolScreeningValidation, applyError)
	}

	return nil
}

// Replacing keeps the stored identity, so an update rewrites the figures of the same profile.
func (profileDomain SymbolScreeningProfileDomain) Replacing(
	stored entities.SymbolScreeningProfile,
) SymbolScreeningProfileDomain {
	profileDomain.profile.ID = stored.ID
	profileDomain.profile.CreatedAt = stored.CreatedAt

	return profileDomain
}

func (profileDomain SymbolScreeningProfileDomain) ToEntity() entities.SymbolScreeningProfile {
	return profileDomain.profile
}

func (profileDomain SymbolScreeningProfileDomain) ToSnapshot(
	runnableScript dto.RunnableStrategyScriptDto,
) dto.SymbolScreeningSnapshotDto {
	profile := profileDomain.profile

	return dto.SymbolScreeningSnapshotDto{
		ProfileName:                    profile.Name,
		StrategyScriptID:               profile.StrategyScriptID,
		Script:                         runnableScript.Script,
		ResultType:                     runnableScript.ResultType,
		Parameters:                     runnableScript.Parameters,
		ParameterValues:                profile.ParameterValues,
		AggregationInterval:            profile.AggregationInterval,
		TradingMode:                    profile.TradingMode,
		Leverage:                       profile.Leverage,
		EntryCostPercentage:            profile.EntryCostPercentage,
		SlippagePercentage:             profile.SlippagePercentage,
		HistoryDays:                    profile.HistoryDays,
		SegmentCount:                   profile.SegmentCount,
		MinimumTradeCount:              profile.MinimumTradeCount,
		MinimumAverageDailyQuoteVolume: profile.MinimumAverageDailyQuoteVolume,
		BaselineSymbols:                profile.BaselineSymbols,
	}
}

package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

var contractCurrentTime = time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)

func tradeCountOf(tradeCount int64) *int64 {
	return &tradeCount
}

func validContractWriteDto() dto.KCandleContractWriteDto {
	return dto.KCandleContractWriteDto{
		Symbol:              "BTCUSDT",
		OpenTime:            time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC),
		Open:                decimal.RequireFromString("85570.30"),
		High:                decimal.RequireFromString("85594.40"),
		Low:                 decimal.RequireFromString("85570.30"),
		Close:               decimal.RequireFromString("85572.00"),
		Volume:              decimal.RequireFromString("55.714"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("4767877.86870")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("34.469")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("2949728.83040")),
		TradeCount:          tradeCountOf(2541),
		MarkOpen:            decimal.NewNullDecimal(decimal.RequireFromString("85573.82868116")),
		MarkHigh:            decimal.NewNullDecimal(decimal.RequireFromString("85594.40000000")),
		MarkLow:             decimal.NewNullDecimal(decimal.RequireFromString("85572.77281159")),
		MarkClose:           decimal.NewNullDecimal(decimal.RequireFromString("85574.49072464")),
	}
}

func TestNewKCandleContractDomainStoresEveryFigureWhenAllAreGiven(t *testing.T) {
	writeDto := validContractWriteDto()

	contractDomain, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

	assert.NoError(t, buildError)
	storedCandle := contractDomain.ToEntity()
	assert.Equal(t, "BTCUSDT", storedCandle.Symbol)
	assert.Equal(t, writeDto.OpenTime, storedCandle.OpenTime)
	assert.True(t, decimal.RequireFromString("85572.00").Equal(storedCandle.Close))
	assert.True(t, decimal.RequireFromString("55.714").Equal(storedCandle.Volume))
	assert.True(t, decimal.RequireFromString("4767877.86870").Equal(storedCandle.QuoteVolume))
	assert.True(t, decimal.RequireFromString("34.469").Equal(storedCandle.TakerBuyBaseVolume))
	assert.True(t, decimal.RequireFromString("2949728.83040").Equal(storedCandle.TakerBuyQuoteVolume))
	assert.Equal(t, int64(2541), storedCandle.TradeCount)
	assert.True(t, decimal.RequireFromString("85573.82868116").Equal(storedCandle.MarkOpen))
	assert.True(t, decimal.RequireFromString("85594.40000000").Equal(storedCandle.MarkHigh))
	assert.True(t, decimal.RequireFromString("85572.77281159").Equal(storedCandle.MarkLow))
	assert.True(t, decimal.RequireFromString("85574.49072464").Equal(storedCandle.MarkClose))
}

func TestNewKCandleContractDomainRejectsAMissingFigure(t *testing.T) {
	testCases := []struct {
		name            string
		breakWriteDto   func(writeDto *dto.KCandleContractWriteDto)
		expectedMessage string
	}{
		{
			name: "沒給標記價格就不是一根合約 K 線",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.MarkClose = decimal.NullDecimal{}
			},
			expectedMessage: "標記價格不得留白",
		},
		{
			name: "沒給成交筆數也拒絕",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.TradeCount = nil
			},
			expectedMessage: "成交筆數不得留白",
		},
		{
			name: "沒給成交額也拒絕",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.QuoteVolume = decimal.NullDecimal{}
			},
			expectedMessage: "成交額不得留白",
		},
		{
			name: "沒給主動買入量也拒絕",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.TakerBuyBaseVolume = decimal.NullDecimal{}
			},
			expectedMessage: "主動買入量不得留白",
		},
		{
			name: "沒給主動買入額也拒絕",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.TakerBuyQuoteVolume = decimal.NullDecimal{}
			},
			expectedMessage: "主動買入額不得留白",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validContractWriteDto()
			testCase.breakWriteDto(&writeDto)

			_, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

			assert.ErrorIs(t, buildError, domains.ErrKCandleContractValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedMessage)
		})
	}
}

func TestNewKCandleContractDomainRejectsAnUnlawfulFigure(t *testing.T) {
	testCases := []struct {
		name            string
		breakWriteDto   func(writeDto *dto.KCandleContractWriteDto)
		expectedMessage string
	}{
		{
			name: "標記的最高價低於最低價",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.MarkHigh = decimal.NewNullDecimal(decimal.RequireFromString("100"))
				writeDto.MarkLow = decimal.NewNullDecimal(decimal.RequireFromString("200"))
			},
			expectedMessage: "標記的最高價不得低於最低價",
		},
		{
			name: "標記價格為負",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.MarkLow = decimal.NewNullDecimal(decimal.RequireFromString("-1"))
			},
			expectedMessage: "標記價格不得為負數",
		},
		{
			name: "成交筆數為負",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.TradeCount = tradeCountOf(-1)
			},
			expectedMessage: "成交筆數不得為負數",
		},
		{
			name: "起始時間指向未來",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.OpenTime = contractCurrentTime.Add(time.Hour)
			},
			expectedMessage: "起始時間不得指向未來",
		},
		{
			name: "起始時間不在一分鐘刻度上",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.OpenTime = writeDto.OpenTime.Add(30 * time.Second)
			},
			expectedMessage: "起始時間必須落在",
		},
		{
			name: "最高價低於最低價",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.High = decimal.RequireFromString("1")
				writeDto.Low = decimal.RequireFromString("2")
			},
			expectedMessage: "最高價不得低於最低價",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validContractWriteDto()
			testCase.breakWriteDto(&writeDto)

			_, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

			assert.ErrorIs(t, buildError, domains.ErrKCandleContractValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedMessage)
		})
	}
}

func TestNewKCandleContractDomainAcceptsAZeroTradeCount(t *testing.T) {
	writeDto := validContractWriteDto()
	writeDto.TradeCount = tradeCountOf(0)

	contractDomain, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

	assert.NoError(t, buildError)
	assert.Equal(t, int64(0), contractDomain.ToEntity().TradeCount)
}

func TestNewKCandleContractDomainAcceptsAMarkPriceFarFromTheLastPrice(t *testing.T) {
	writeDto := validContractWriteDto()
	writeDto.Open = decimal.RequireFromString("100")
	writeDto.High = decimal.RequireFromString("100")
	writeDto.Low = decimal.RequireFromString("100")
	writeDto.Close = decimal.RequireFromString("100")
	writeDto.MarkOpen = decimal.NewNullDecimal(decimal.RequireFromString("100000"))
	writeDto.MarkHigh = decimal.NewNullDecimal(decimal.RequireFromString("100000"))
	writeDto.MarkLow = decimal.NewNullDecimal(decimal.RequireFromString("100000"))
	writeDto.MarkClose = decimal.NewNullDecimal(decimal.RequireFromString("100000"))

	contractDomain, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

	assert.NoError(t, buildError)
	storedCandle := contractDomain.ToEntity()
	assert.True(t, decimal.RequireFromString("100").Equal(storedCandle.Close))
	assert.True(t, decimal.RequireFromString("100000").Equal(storedCandle.MarkClose))
}

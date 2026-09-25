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
		IndexOpen:           decimal.NewNullDecimal(decimal.RequireFromString("87248.06")),
		IndexHigh:           decimal.NewNullDecimal(decimal.RequireFromString("87268.33")),
		IndexLow:            decimal.NewNullDecimal(decimal.RequireFromString("87220.35")),
		IndexClose:          decimal.NewNullDecimal(decimal.RequireFromString("87261.48")),
		PremiumIndexOpen:    decimal.NewNullDecimal(decimal.RequireFromString("0.0001")),
		PremiumIndexHigh:    decimal.NewNullDecimal(decimal.RequireFromString("0.0003")),
		PremiumIndexLow:     decimal.NewNullDecimal(decimal.RequireFromString("0.0000")),
		PremiumIndexClose:   decimal.NewNullDecimal(decimal.RequireFromString("0.0002")),
	}
}

func figure(value string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(value))
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

func TestNewKCandleContractDomainKeepsTheIndexPriceAndPremiumIndexAsGiven(t *testing.T) {
	testCases := []struct {
		name                                               string
		premiumOpen, premiumHigh, premiumLow, premiumClose string
	}{
		{name: "合約比現貨便宜,溢價指數為負", premiumOpen: "-0.0005", premiumHigh: "-0.0003",
			premiumLow: "-0.0007", premiumClose: "-0.0005"},
		{name: "溢價指數四個都是零", premiumOpen: "0", premiumHigh: "0", premiumLow: "0", premiumClose: "0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validContractWriteDto()
			writeDto.PremiumIndexOpen = figure(testCase.premiumOpen)
			writeDto.PremiumIndexHigh = figure(testCase.premiumHigh)
			writeDto.PremiumIndexLow = figure(testCase.premiumLow)
			writeDto.PremiumIndexClose = figure(testCase.premiumClose)

			contractDomain, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

			assert.NoError(t, buildError)
			storedCandle := contractDomain.ToEntity()
			assert.True(t, decimal.RequireFromString(testCase.premiumOpen).Equal(storedCandle.PremiumIndexOpen.Decimal))
			assert.True(t, decimal.RequireFromString(testCase.premiumHigh).Equal(storedCandle.PremiumIndexHigh.Decimal))
			assert.True(t, decimal.RequireFromString(testCase.premiumLow).Equal(storedCandle.PremiumIndexLow.Decimal))
			assert.True(t, decimal.RequireFromString(testCase.premiumClose).Equal(storedCandle.PremiumIndexClose.Decimal))
			assert.True(t, storedCandle.PremiumIndexClose.Valid)
			assert.True(t, decimal.RequireFromString("87248.06").Equal(storedCandle.IndexOpen.Decimal))
			assert.True(t, decimal.RequireFromString("87268.33").Equal(storedCandle.IndexHigh.Decimal))
			assert.True(t, decimal.RequireFromString("87220.35").Equal(storedCandle.IndexLow.Decimal))
			assert.True(t, decimal.RequireFromString("87261.48").Equal(storedCandle.IndexClose.Decimal))
			assert.True(t, storedCandle.IndexClose.Valid)
		})
	}
}

func TestNewKCandleContractDomainJudgesTheIndexPriceAndPremiumIndex(t *testing.T) {
	testCases := []struct {
		name            string
		breakWriteDto   func(writeDto *dto.KCandleContractWriteDto)
		expectedMessage string
	}{
		{
			name: "溢價指數高低於低",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.PremiumIndexHigh = figure("-0.0008")
				writeDto.PremiumIndexLow = figure("-0.0003")
			},
			expectedMessage: "溢價指數最高不得低於最低",
		},
		{
			name: "指數價格的低為負",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.IndexLow = figure("-1")
			},
			expectedMessage: "指數價格不得為負",
		},
		{
			name: "指數價格的開盤為負,最低不是負的",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.IndexOpen = figure("-1")
			},
			expectedMessage: "指數價格不得為負",
		},
		{
			name: "指數價格高低於低",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.IndexHigh = figure("86000")
				writeDto.IndexLow = figure("87000")
			},
			expectedMessage: "指數價格最高不得低於最低",
		},
		{
			name: "沒給指數價格",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.IndexClose = decimal.NullDecimal{}
			},
			expectedMessage: "指數價格必填",
		},
		{
			name: "沒給溢價指數",
			breakWriteDto: func(writeDto *dto.KCandleContractWriteDto) {
				writeDto.PremiumIndexOpen = decimal.NullDecimal{}
			},
			expectedMessage: "溢價指數必填",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := validContractWriteDto()
			testCase.breakWriteDto(&writeDto)

			_, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

			assert.ErrorIs(t, buildError, domains.ErrKCandleContractValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestNewKCandleContractDomainAcceptsAnIndexPriceFarFromTheLastPrice(t *testing.T) {
	writeDto := validContractWriteDto()
	writeDto.Close = decimal.RequireFromString("90000")
	writeDto.High = decimal.RequireFromString("90000")
	writeDto.IndexClose = figure("87000")

	_, buildError := domains.NewKCandleContractDomain(writeDto, contractCurrentTime)

	assert.NoError(t, buildError)
}

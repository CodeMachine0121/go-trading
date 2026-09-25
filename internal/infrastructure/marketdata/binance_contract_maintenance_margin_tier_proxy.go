package marketdata

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// signedRequestWindow is the venue's default recvWindow, stated explicitly.
const signedRequestWindow = 5 * time.Second

// binanceLeverageBracket keeps numbers as json.Number text so they never pass through float64.
type binanceLeverageBracket struct {
	Symbol   string `json:"symbol"`
	Brackets []struct {
		Bracket          int         `json:"bracket"`
		InitialLeverage  int         `json:"initialLeverage"`
		NotionalCap      json.Number `json:"notionalCap"`
		NotionalFloor    json.Number `json:"notionalFloor"`
		MaintMarginRatio json.Number `json:"maintMarginRatio"`
		Cum              json.Number `json:"cum"`
	} `json:"brackets"`
}

// BinanceContractMaintenanceMarginTierProxy fetches every contract's maintenance margin ladder with a signed request; the key and secret are never written into errors.
type BinanceContractMaintenanceMarginTierProxy struct {
	leverageBracketUrl string
	apiKey             string
	apiSecret          string
	httpClient         *http.Client
	pacer              RequestPacer
	clockProxy         domaininterface.IClockProxy
}

func NewBinanceContractMaintenanceMarginTierProxy(
	leverageBracketUrl string,
	apiKey string,
	apiSecret string,
	requestTimeout time.Duration,
	pacer RequestPacer,
	clockProxy domaininterface.IClockProxy,
) *BinanceContractMaintenanceMarginTierProxy {
	return &BinanceContractMaintenanceMarginTierProxy{
		leverageBracketUrl: leverageBracketUrl,
		apiKey:             apiKey,
		apiSecret:          apiSecret,
		httpClient:         &http.Client{Timeout: requestTimeout},
		pacer:              pacer,
		clockProxy:         clockProxy,
	}
}

func (tierProxy *BinanceContractMaintenanceMarginTierProxy) FetchMaintenanceMarginLadders(
	executionContext context.Context,
) ([]vo.ContractMaintenanceMarginLadderVo, error) {
	if tierProxy.apiKey == "" || tierProxy.apiSecret == "" {
		return nil, domains.ErrContractAccountCredentialsMissing
	}

	if waitError := tierProxy.pacer.WaitForTurn(executionContext); waitError != nil {
		return nil, waitError
	}

	queryValues := url.Values{}
	queryValues.Set("timestamp", strconv.FormatInt(tierProxy.clockProxy.Now().UnixMilli(), 10))
	queryValues.Set("recvWindow", strconv.FormatInt(signedRequestWindow.Milliseconds(), 10))
	unsignedQuery := queryValues.Encode()
	signer := hmac.New(sha256.New, []byte(tierProxy.apiSecret))
	signer.Write([]byte(unsignedQuery))
	signedQuery := unsignedQuery + "&signature=" + hex.EncodeToString(signer.Sum(nil))

	request, buildError := http.NewRequestWithContext(executionContext, http.MethodGet,
		tierProxy.leverageBracketUrl+"?"+signedQuery, nil)
	if buildError != nil {
		return nil, fmt.Errorf("reach contract account source: %w", buildError)
	}
	request.Header.Set("X-MBX-APIKEY", tierProxy.apiKey)

	response, requestError := tierProxy.httpClient.Do(request)
	if requestError != nil {
		return nil, fmt.Errorf("reach contract account source: %w", requestError)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: contract account source answered %d",
			domains.ErrContractAccountCredentialsRefused, response.StatusCode)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("contract account source answered %d", response.StatusCode)
	}

	var reportedBrackets []binanceLeverageBracket
	if decodeError := json.NewDecoder(response.Body).Decode(&reportedBrackets); decodeError != nil {
		return nil, fmt.Errorf("read contract account source answer: %w", decodeError)
	}

	ladders := make([]vo.ContractMaintenanceMarginLadderVo, 0, len(reportedBrackets))
	for _, reportedBracket := range reportedBrackets {
		ladder, convertError := reportedBracket.toContractMaintenanceMarginLadderVo()
		if convertError != nil {
			return nil, convertError
		}
		ladders = append(ladders, ladder)
	}

	return ladders, nil
}

// toContractMaintenanceMarginLadderVo maps the venue's "cum" to the tier's fixed deduction and "initialLeverage" to its maximum leverage.
func (reportedBracket binanceLeverageBracket) toContractMaintenanceMarginLadderVo() (
	vo.ContractMaintenanceMarginLadderVo, error,
) {
	tiers := make([]vo.ContractMaintenanceMarginTierVo, 0, len(reportedBracket.Brackets))
	for _, bracket := range reportedBracket.Brackets {
		tier := vo.ContractMaintenanceMarginTierVo{
			Tier: bracket.Bracket, MaximumLeverage: bracket.InitialLeverage,
		}
		for _, figure := range []struct {
			name   string
			number json.Number
			target *decimal.Decimal
		}{
			{"notionalFloor", bracket.NotionalFloor, &tier.NotionalFloor},
			{"notionalCap", bracket.NotionalCap, &tier.NotionalCap},
			{"maintMarginRatio", bracket.MaintMarginRatio, &tier.MaintenanceMarginRate},
			{"cum", bracket.Cum, &tier.MaintenanceAmount},
		} {
			value, parseError := decimal.NewFromString(figure.number.String())
			if parseError != nil {
				return vo.ContractMaintenanceMarginLadderVo{}, fmt.Errorf(
					"read %s of %s from contract account source: %w",
					figure.name, reportedBracket.Symbol, parseError)
			}
			*figure.target = value
		}
		tiers = append(tiers, tier)
	}

	return vo.ContractMaintenanceMarginLadderVo{Symbol: reportedBracket.Symbol, Tiers: tiers}, nil
}

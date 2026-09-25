package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadReadsTheRequestLimits(t *testing.T) {
	defaults := config.RequestLimitConfig{
		TrustedProxyCidrs:           []string{},
		ClientIpHeaders:             []string{"X-Forwarded-For", "X-Real-IP"},
		BodyLimitBytes:              1024 << 10,
		ReadTimeout:                 30 * time.Second,
		IdleTimeout:                 120 * time.Second,
		RequestsPerMinute:           600,
		RequestBurst:                120,
		CredentialRequestsPerMinute: 10,
		CredentialRequestBurst:      10,
		LiveStreamsPerClient:        20,
		LiveStreamsTotal:            1000,
	}

	testCases := []struct {
		name        string
		environment map[string]string
		expected    config.RequestLimitConfig
	}{
		{name: "nothing set gives the defaults", expected: defaults},
		{
			name: "usable values are taken as given",
			environment: map[string]string{
				"TRUSTED_PROXY_CIDRS":                       "10.42.0.0/16, 10.43.0.0/16",
				"CLIENT_IP_HEADERS":                         "CF-Connecting-IP,X-Forwarded-For",
				"REQUEST_BODY_LIMIT_KILOBYTES":              "2048",
				"SERVER_READ_TIMEOUT_SECONDS":               "15",
				"SERVER_IDLE_TIMEOUT_SECONDS":               "300",
				"RATE_LIMIT_REQUESTS_PER_MINUTE":            "1200",
				"RATE_LIMIT_BURST":                          "200",
				"RATE_LIMIT_CREDENTIAL_REQUESTS_PER_MINUTE": "5",
				"RATE_LIMIT_CREDENTIAL_BURST":               "3",
				"LIVE_STREAM_CONNECTIONS_PER_CLIENT":        "8",
				"LIVE_STREAM_CONNECTIONS_TOTAL":             "200",
			},
			expected: config.RequestLimitConfig{
				TrustedProxyCidrs:           []string{"10.42.0.0/16", "10.43.0.0/16"},
				ClientIpHeaders:             []string{"CF-Connecting-IP", "X-Forwarded-For"},
				BodyLimitBytes:              2048 << 10,
				ReadTimeout:                 15 * time.Second,
				IdleTimeout:                 300 * time.Second,
				RequestsPerMinute:           1200,
				RequestBurst:                200,
				CredentialRequestsPerMinute: 5,
				CredentialRequestBurst:      3,
				LiveStreamsPerClient:        8,
				LiveStreamsTotal:            200,
			},
		},
		{
			name: "zero, negative and unreadable values fall back",
			environment: map[string]string{
				"REQUEST_BODY_LIMIT_KILOBYTES":              "0",
				"SERVER_READ_TIMEOUT_SECONDS":               "-1",
				"SERVER_IDLE_TIMEOUT_SECONDS":               "forever",
				"RATE_LIMIT_REQUESTS_PER_MINUTE":            "0",
				"RATE_LIMIT_BURST":                          "-5",
				"RATE_LIMIT_CREDENTIAL_REQUESTS_PER_MINUTE": "many",
				"RATE_LIMIT_CREDENTIAL_BURST":               "0",
				"LIVE_STREAM_CONNECTIONS_PER_CLIENT":        "-1",
				"LIVE_STREAM_CONNECTIONS_TOTAL":             "lots",
			},
			expected: defaults,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			for key, value := range testCase.environment {
				t.Setenv(key, value)
			}

			assert.Equal(t, testCase.expected, config.Load().RequestLimit)
		})
	}
}

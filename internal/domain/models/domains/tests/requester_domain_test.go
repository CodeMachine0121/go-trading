package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

func TestRequesterCountsAgainstTheUserOtherwiseTheAddress(t *testing.T) {
	testCases := []struct {
		name          string
		first         domains.RequesterDomain
		second        domains.RequesterDomain
		expectSameKey bool
	}{
		{
			name:          "an identified user is the same requester from any address",
			first:         domains.NewRequesterDomain(7, "203.0.113.5"),
			second:        domains.NewRequesterDomain(7, "198.51.100.9"),
			expectSameKey: true,
		},
		{
			name:          "an unidentified request counts against its address",
			first:         domains.NewRequesterDomain(0, "203.0.113.5"),
			second:        domains.NewRequesterDomain(0, "203.0.113.5"),
			expectSameKey: true,
		},
		{
			name:          "two users behind one address are two requesters",
			first:         domains.NewRequesterDomain(7, "203.0.113.5"),
			second:        domains.NewRequesterDomain(8, "203.0.113.5"),
			expectSameKey: false,
		},
		{
			name:          "a user is never the anonymous requester at the same address",
			first:         domains.NewRequesterDomain(7, "203.0.113.5"),
			second:        domains.NewRequesterDomain(0, "203.0.113.5"),
			expectSameKey: false,
		},
		{
			name:          "a user id never collides with an address spelled the same",
			first:         domains.NewRequesterDomain(7, "203.0.113.5"),
			second:        domains.NewRequesterDomain(0, "7"),
			expectSameKey: false,
		},
		{
			name:          "neighbouring addresses in one /64 block are one source",
			first:         domains.NewRequesterDomain(0, "2001:db8:1:2:aaaa::1"),
			second:        domains.NewRequesterDomain(0, "2001:db8:1:2:bbbb::9"),
			expectSameKey: true,
		},
		{
			name:          "addresses in different /64 blocks are different sources",
			first:         domains.NewRequesterDomain(0, "2001:db8:1:2::1"),
			second:        domains.NewRequesterDomain(0, "2001:db8:1:3::1"),
			expectSameKey: false,
		},
		{
			name:          "an IPv4 address written as IPv6 is the same source",
			first:         domains.NewRequesterDomain(0, "::ffff:203.0.113.5"),
			second:        domains.NewRequesterDomain(0, "203.0.113.5"),
			expectSameKey: true,
		},
		{
			name:          "an unreadable address is still told apart from others",
			first:         domains.NewRequesterDomain(0, "not-an-address"),
			second:        domains.NewRequesterDomain(0, "203.0.113.5"),
			expectSameKey: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectSameKey, testCase.first.Key() == testCase.second.Key())
		})
	}
}

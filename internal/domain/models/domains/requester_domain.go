package domains

import (
	"net/netip"
	"strconv"
)

// ipv6NeighbourhoodBits is the block one household is usually given; counting finer would hand out a fresh allowance per address.
const ipv6NeighbourhoodBits = 64

// RequesterDomain decides whom a request counts against: the identified user, otherwise the source address.
type RequesterDomain struct {
	userID     uint
	addressKey string
}

// NewRequesterDomain takes zero as "not identified"; an unreadable address is still counted, just verbatim.
func NewRequesterDomain(userID uint, clientAddress string) RequesterDomain {
	address, parseError := netip.ParseAddr(clientAddress)
	if parseError != nil {
		return RequesterDomain{userID: userID, addressKey: "address:" + clientAddress}
	}

	address = address.Unmap().WithZone("")
	if address.Is6() {
		neighbourhood, _ := address.Prefix(ipv6NeighbourhoodBits)

		return RequesterDomain{userID: userID, addressKey: "address:" + neighbourhood.String()}
	}

	return RequesterDomain{userID: userID, addressKey: "address:" + address.String()}
}

func (requesterDomain RequesterDomain) Key() string {
	if requesterDomain.userID > 0 {
		return strconv.FormatUint(uint64(requesterDomain.userID), 10)
	}

	return requesterDomain.addressKey
}

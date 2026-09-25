package service

import (
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// RequestAdmissionService decides whether a requester may be served right now; the state lives in memory
// because the service always runs as a single replica.
type RequestAdmissionService struct {
	accessTokenProxy            domaininterface.IAccessTokenProxy
	clockProxy                  domaininterface.IClockProxy
	requestAllowances           *domains.RequestAllowanceLedgerDomain
	credentialRequestAllowances *domains.RequestAllowanceLedgerDomain
	liveStreamOccupancy         *domains.LiveStreamOccupancyDomain
}

func NewRequestAdmissionService(
	accessTokenProxy domaininterface.IAccessTokenProxy,
	clockProxy domaininterface.IClockProxy,
	requestBudget vo.RequestBudgetVo,
	credentialRequestBudget vo.RequestBudgetVo,
	liveStreamCapacity vo.LiveStreamCapacityVo,
) *RequestAdmissionService {
	return &RequestAdmissionService{
		accessTokenProxy:            accessTokenProxy,
		clockProxy:                  clockProxy,
		requestAllowances:           domains.NewRequestAllowanceLedgerDomain(requestBudget),
		credentialRequestAllowances: domains.NewRequestAllowanceLedgerDomain(credentialRequestBudget),
		liveStreamOccupancy:         domains.NewLiveStreamOccupancyDomain(liveStreamCapacity),
	}
}

func (requestAdmissionService *RequestAdmissionService) AdmitRequest(requesterDto dto.RequesterDto) error {
	return requestAdmissionService.requestAllowances.Admit(
		requestAdmissionService.requesterOf(requesterDto).Key(), requestAdmissionService.clockProxy.Now())
}

// AdmitCredentialRequest ignores the access token, otherwise registering accounts would buy fresh allowances.
func (requestAdmissionService *RequestAdmissionService) AdmitCredentialRequest(requesterDto dto.RequesterDto) error {
	return requestAdmissionService.credentialRequestAllowances.Admit(
		domains.NewRequesterDomain(0, requesterDto.ClientAddress).Key(), requestAdmissionService.clockProxy.Now())
}

func (requestAdmissionService *RequestAdmissionService) OpenLiveStream(
	requesterDto dto.RequesterDto,
) (dto.LiveStreamSlotDto, error) {
	requesterKey := requestAdmissionService.requesterOf(requesterDto).Key()
	if occupyError := requestAdmissionService.liveStreamOccupancy.Occupy(requesterKey); occupyError != nil {
		return dto.LiveStreamSlotDto{}, occupyError
	}

	return dto.LiveStreamSlotDto{RequesterKey: requesterKey}, nil
}

func (requestAdmissionService *RequestAdmissionService) CloseLiveStream(liveStreamSlotDto dto.LiveStreamSlotDto) {
	requestAdmissionService.liveStreamOccupancy.Vacate(liveStreamSlotDto.RequesterKey)
}

// requesterOf checks only the token's signature and expiry, since every request pays for it; a token that
// fails falls back to the address, so nobody can spend another user's allowance.
func (requestAdmissionService *RequestAdmissionService) requesterOf(requesterDto dto.RequesterDto) domains.RequesterDomain {
	if requesterDto.AccessToken == "" {
		return domains.NewRequesterDomain(0, requesterDto.ClientAddress)
	}

	userID, identifyError := requestAdmissionService.accessTokenProxy.UserIdentifiedBy(requesterDto.AccessToken)
	if identifyError != nil {
		return domains.NewRequesterDomain(0, requesterDto.ClientAddress)
	}

	return domains.NewRequesterDomain(userID, requesterDto.ClientAddress)
}

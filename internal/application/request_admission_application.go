package application

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type RequestAdmissionApplication struct {
	requestAdmissionService *service.RequestAdmissionService
}

func NewRequestAdmissionApplication(requestAdmissionService *service.RequestAdmissionService) *RequestAdmissionApplication {
	return &RequestAdmissionApplication{requestAdmissionService: requestAdmissionService}
}

func (requestAdmissionApplication *RequestAdmissionApplication) AdmitRequest(requesterDto dto.RequesterDto) error {
	return requestAdmissionApplication.requestAdmissionService.AdmitRequest(requesterDto)
}

func (requestAdmissionApplication *RequestAdmissionApplication) AdmitCredentialRequest(requesterDto dto.RequesterDto) error {
	return requestAdmissionApplication.requestAdmissionService.AdmitCredentialRequest(requesterDto)
}

func (requestAdmissionApplication *RequestAdmissionApplication) OpenLiveStream(
	requesterDto dto.RequesterDto,
) (dto.LiveStreamSlotDto, error) {
	return requestAdmissionApplication.requestAdmissionService.OpenLiveStream(requesterDto)
}

func (requestAdmissionApplication *RequestAdmissionApplication) CloseLiveStream(liveStreamSlotDto dto.LiveStreamSlotDto) {
	requestAdmissionApplication.requestAdmissionService.CloseLiveStream(liveStreamSlotDto)
}

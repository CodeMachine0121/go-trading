package domains

// SymbolScreeningAdmissionDomain lets one administrator have a single run queued or running, since the duty replica screens one run at a time.
type SymbolScreeningAdmissionDomain struct{}

func NewSymbolScreeningAdmissionDomain() SymbolScreeningAdmissionDomain {
	return SymbolScreeningAdmissionDomain{}
}

func (admissionDomain SymbolScreeningAdmissionDomain) Admit(ownerActiveCount int) error {
	if ownerActiveCount > 0 {
		return ErrSymbolScreeningRunAlreadyActive
	}

	return nil
}

package domains

// BacktestRunAdmissionDomain is the per-owner cap on runs queued or running at once: the queue is shared, so one person cannot fill it.
type BacktestRunAdmissionDomain struct {
	ownerActiveLimit int
}

const defaultBacktestRunOwnerActiveLimit = 3

func NewBacktestRunAdmissionDomain(ownerActiveLimit int) BacktestRunAdmissionDomain {
	if ownerActiveLimit <= 0 {
		ownerActiveLimit = defaultBacktestRunOwnerActiveLimit
	}

	return BacktestRunAdmissionDomain{ownerActiveLimit: ownerActiveLimit}
}

func (admissionDomain BacktestRunAdmissionDomain) Admit(ownerActiveCount int) error {
	if ownerActiveCount >= admissionDomain.ownerActiveLimit {
		return &backtestRunOwnerLimitError{limit: admissionDomain.ownerActiveLimit}
	}

	return nil
}

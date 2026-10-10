package domains

// BacktestRunAdmissionDomain is the per-owner cap on runs queued or running at once: the queue is shared, so one person cannot fill it.
type BacktestRunAdmissionDomain struct {
	ownerActiveLimit int
}

const defaultBacktestRunOwnerActiveLimit = 3

// NewBacktestRunAdmissionDomain falls back to the default for a limit that is not a positive number.
func NewBacktestRunAdmissionDomain(ownerActiveLimit int) BacktestRunAdmissionDomain {
	if ownerActiveLimit <= 0 {
		ownerActiveLimit = defaultBacktestRunOwnerActiveLimit
	}

	return BacktestRunAdmissionDomain{ownerActiveLimit: ownerActiveLimit}
}

// Admit refuses once the owner's active count has reached the limit, naming the limit so the person knows what to wait for.
func (admissionDomain BacktestRunAdmissionDomain) Admit(ownerActiveCount int) error {
	if ownerActiveCount >= admissionDomain.ownerActiveLimit {
		return &backtestRunOwnerLimitError{limit: admissionDomain.ownerActiveLimit}
	}

	return nil
}

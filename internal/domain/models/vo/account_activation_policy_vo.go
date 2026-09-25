package vo

// AccountActivationPolicyVo is where activation requests are mailed and how their subject starts; both have defaults because neither is a secret.
type AccountActivationPolicyVo struct {
	RequestMailbox string
	// SubjectPrefix is followed by the applicant's email so the inbox shows who is asking.
	SubjectPrefix string
}

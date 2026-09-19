package vo

// AccountActivationPolicyVo is where an activation request has to be sent and how its
// subject line has to start.
//
// It is a setting rather than a constant because it is a fact about somebody's inbox,
// not about this system: a test points it at a stand-in, and the person running this
// console points it at their own address.
//
// Unlike the two keys this system reads from its environment, both halves have a
// default. Neither is a key: a mailbox anybody can guess gives nothing away, whereas
// refusing to start for want of one would take the whole console down over a setting
// that guards nothing.
type AccountActivationPolicyVo struct {
	// RequestMailbox is the address a person asking to be let in writes to.
	RequestMailbox string
	// SubjectPrefix is the fixed opening of that letter's subject. The rest of the
	// subject is the applicant's own email address, so that the person reading the
	// inbox can tell who is asking from the list alone.
	SubjectPrefix string
}

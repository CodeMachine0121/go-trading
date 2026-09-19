package controller_test

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// testActivationPolicy is where these tests pretend letters asking to be let in are
// sent. It is a stand-in address on purpose: a test that named the real inbox would
// read as if it were about that inbox, when what it is about is that the subject line
// is assembled from whatever the setting says.
var testActivationPolicy = vo.AccountActivationPolicyVo{
	RequestMailbox: "gatekeeper@example.com",
	SubjectPrefix:  "console access request",
}

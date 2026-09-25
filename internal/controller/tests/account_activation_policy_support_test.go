package controller_test

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// testActivationPolicy uses a stand-in address: the tests are about the subject being assembled from the setting.
var testActivationPolicy = vo.AccountActivationPolicyVo{
	RequestMailbox: "gatekeeper@example.com",
	SubjectPrefix:  "console access request",
}

package security_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBcryptPasswordProofProxyProveNeverHandsBackThePassword(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	passwordProof, err := passwordProofProxy.Prove("correct horse")

	require.NoError(t, err)
	assert.NotEqual(t, "correct horse", passwordProof)
	assert.NotContains(t, passwordProof, "correct horse")
}

func TestBcryptPasswordProofProxyProveGivesADifferentProofEveryTime(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	firstProof, firstError := passwordProofProxy.Prove("correct horse")
	secondProof, secondError := passwordProofProxy.Prove("correct horse")

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.NotEqual(t, firstProof, secondProof,
		"兩個人剛好用同一組密碼時，留存的內容也不得看得出他們用的是同一組")
	assert.True(t, passwordProofProxy.Matches("correct horse", firstProof))
	assert.True(t, passwordProofProxy.Matches("correct horse", secondProof))
}

func TestBcryptPasswordProofProxyMatches(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()
	passwordProof, err := passwordProofProxy.Prove("correct horse")
	require.NoError(t, err)

	testCases := []struct {
		name            string
		password        string
		passwordProof   string
		expectedMatches bool
	}{
		{
			name:            "the password it was derived from",
			password:        "correct horse",
			passwordProof:   passwordProof,
			expectedMatches: true,
		},
		{
			name:            "some other password",
			password:        "wrong horse",
			passwordProof:   passwordProof,
			expectedMatches: false,
		},
		{
			name:            "a proof it cannot even read is a no, not a crash",
			password:        "correct horse",
			passwordProof:   "not-a-proof",
			expectedMatches: false,
		},
		{
			name:            "no proof at all, which is what having no account looks like",
			password:        "correct horse",
			passwordProof:   "",
			expectedMatches: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedMatches,
				passwordProofProxy.Matches(testCase.password, testCase.passwordProof))
		})
	}
}

func TestBcryptPasswordProofProxyProveRefusesAPasswordItCouldOnlyReadTheFrontOf(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	_, err := passwordProofProxy.Prove(strings.Repeat("a", 73))

	require.Error(t, err, "只讀得完前面一段的密碼，存成證明就是在騙它的主人")
}

// minimumRefusalEffort is well under what one derivation costs and enormously more
// than returning early costs, so this reads as "it did the work" without pinning the
// test to any particular machine's speed.
const minimumRefusalEffort = 10 * time.Millisecond

func TestBcryptPasswordProofProxyRefusingNoProofCostsWhatRefusingAWrongOneCosts(t *testing.T) {
	// This is the whole reason a decoy exists. Somebody signing in with an address
	// nobody holds has no proof to be checked against, and refusing that instantly
	// answers "not registered" in the one thing nobody thought to hide: how long it
	// took.
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	startedAt := time.Now()
	matches := passwordProofProxy.Matches("correct horse", "")
	refusalEffort := time.Since(startedAt)

	assert.False(t, matches)
	assert.Greater(t, refusalEffort, minimumRefusalEffort,
		"沒有證明可比對時直接回絕，會比密碼打錯快上好幾個數量級——那個時間差就是答案")
}

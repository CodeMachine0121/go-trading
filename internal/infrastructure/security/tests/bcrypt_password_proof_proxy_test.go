package security_test

import (
	"strings"
	"testing"

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
			name:            "no proof at all",
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

func TestBcryptPasswordProofProxyDecoyProofIsAProofNoPasswordMatches(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	decoyProof := passwordProofProxy.DecoyProof()

	assert.NotEmpty(t, decoyProof)
	assert.False(t, passwordProofProxy.Matches("correct horse", decoyProof))
	assert.False(t, passwordProofProxy.Matches("", decoyProof))
}

func TestBcryptPasswordProofProxyDecoyProofIsDerivedOnceAndKept(t *testing.T) {
	passwordProofProxy := security.NewBcryptPasswordProofProxy()

	assert.Equal(t, passwordProofProxy.DecoyProof(), passwordProofProxy.DecoyProof(),
		"每次重算等於在最慢的那條路上把成本再付一次")
}

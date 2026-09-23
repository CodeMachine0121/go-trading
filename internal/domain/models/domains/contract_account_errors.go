package domains

import "errors"

// ErrContractAccountCredentialsMissing marks a question that needs the account's key
// and none is configured. It is not a failure: the system runs without one, and a
// caller that meets this simply goes without what only an account can see.
var ErrContractAccountCredentialsMissing = errors.New("contract account credentials are not configured")

// ErrContractAccountCredentialsRefused marks the venue refusing the account's key —
// wrong, expired, or without the permission asked for.
var ErrContractAccountCredentialsRefused = errors.New("contract account credentials were refused")

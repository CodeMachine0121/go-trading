package domains

import "errors"

// ErrContractAccountCredentialsMissing is not a failure: the system runs without account keys and callers simply skip account-only data.
var ErrContractAccountCredentialsMissing = errors.New("contract account credentials are not configured")

// ErrContractAccountCredentialsRefused covers a wrong, expired or under-permissioned key.
var ErrContractAccountCredentialsRefused = errors.New("contract account credentials were refused")

package platform

import "fmt"

// CredentialNotConfiguredError identifies a platform operation which requires
// an administrator-provided credential before it can contact its upstream.
type CredentialNotConfiguredError struct {
	Platform string
}

func (e *CredentialNotConfiguredError) Error() string {
	return fmt.Sprintf("%s credential is not configured", e.Platform)
}

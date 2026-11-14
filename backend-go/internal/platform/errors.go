package platform

import "fmt"

// CredentialNotConfiguredError identifies a platform operation which requires
// an administrator-provided credential before it can contact its upstream.
type CredentialNotConfiguredError struct {
	Platform string
}

// StreamResolutionError describes a track URL that cannot represent a full stream.
type StreamResolutionError struct {
	Kind   string
	Reason int
}

const (
	StreamTrialOnly   = "trial-only"
	StreamUnavailable = "unavailable"
)

func (e *StreamResolutionError) Error() string {
	if e.Kind == StreamTrialOnly {
		return "only a trial preview is available"
	}
	return "stream is unavailable"
}

// StreamInvalidator is an optional capability for providers with expiring upstream URLs.
type StreamInvalidator interface {
	InvalidateStream(string)
}

func (e *CredentialNotConfiguredError) Error() string {
	return fmt.Sprintf("%s credential is not configured", e.Platform)
}

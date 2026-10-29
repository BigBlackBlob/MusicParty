package platform

import "strings"

const (
	MaxOffset              = 10000
	MaxSearchLimit         = 100
	MaxPlaylistLimit       = 500
	MaxPlaylistImportLimit = MaxOffset + MaxPlaylistLimit
)

// InputError distinguishes invalid caller input from an upstream failure.
type InputError struct{ Message string }

func (e *InputError) Error() string { return e.Message }

func ValidatePage(offset, limit, maxLimit int) error {
	if offset < 0 || offset > MaxOffset || limit < 1 || limit > maxLimit {
		return &InputError{Message: "Invalid pagination parameters"}
	}
	return nil
}

func ValidateKeyword(keyword string) error {
	if strings.TrimSpace(keyword) == "" || len(keyword) > 128 {
		return &InputError{Message: "Invalid search keyword"}
	}
	return nil
}

func ValidateSearch(keyword string, offset, limit int) error {
	if err := ValidatePage(offset, limit, MaxSearchLimit); err != nil {
		return err
	}
	return ValidateKeyword(keyword)
}

// The web album endpoint historically uses (0, 0) for the upstream default page.
func ValidateAlbumSearch(keyword string, offset, limit int) error {
	if offset == 0 && limit == 0 {
		return ValidateKeyword(keyword)
	}
	return ValidateSearch(keyword, offset, limit)
}

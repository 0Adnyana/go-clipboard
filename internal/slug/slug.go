package slug

import (
	"fmt"
	"regexp"
)

const (
	MinLength = 3
	MaxLength = 64
)

var pattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Reserved lists top-level path segments that must not be claimable as clips.
// Keep in sync with fixed frontend routes (see tasks 4.2).
var Reserved = []string{
	"status",
}

// ValidationError is returned when a slug fails charset, length, or reservation rules.
type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// IsReserved reports whether slug exactly matches a reserved name (case-sensitive).
func IsReserved(slug string) bool {
	for _, name := range Reserved {
		if slug == name {
			return true
		}
	}
	return false
}

// Validate checks slug charset, length, and reservation. The slug string is preserved
// as-is on success; validation does not alter casing.
func Validate(slug string) error {
	if len(slug) < MinLength || len(slug) > MaxLength {
		return &ValidationError{
			Code:    "invalid_slug",
			Message: fmt.Sprintf("slug must be %d–%d characters", MinLength, MaxLength),
		}
	}
	if !pattern.MatchString(slug) {
		return &ValidationError{
			Code:    "invalid_slug",
			Message: "slug may only contain letters, digits, underscore, and hyphen",
		}
	}
	if IsReserved(slug) {
		return &ValidationError{
			Code:    "reserved_slug",
			Message: "that name is reserved",
		}
	}
	return nil
}

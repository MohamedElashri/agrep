package arabic

import "errors"

// ErrEmptyKey is returned by callers that reject a value which normalizes to
// the empty string, such as a search query consisting only of tashkil.
var ErrEmptyKey = errors.New("arabic: value is empty after normalization")

// Normalize returns a comparison key for s under ProfileSearch, agrep's
// original, default behavior. It is a convenience for the common case; use
// Profile.Normalize directly to select a different profile.
func Normalize(s string) string {
	return ProfileSearch.Normalize(s)
}

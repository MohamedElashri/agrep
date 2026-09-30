package update

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// Version represents a parsed Semantic Version (SemVer 2.0.0).
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
	Raw        string
}

func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	if v.Build != "" {
		s += "+" + v.Build
	}
	return s
}

// IsDev reports whether a version string denotes a development or unversioned build.
func IsDev(v string) bool {
	s := strings.TrimSpace(strings.ToLower(v))
	return s == "" || s == "dev" || s == "(devel)" || strings.HasPrefix(s, "dev-") || strings.Contains(s, "-dev")
}

// NormalizeTag ensures the version string has a leading "v".
func NormalizeTag(v string) string {
	s := strings.TrimSpace(v)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "v") && !strings.HasPrefix(s, "V") {
		return "v" + s
	}
	return "v" + s[1:]
}

// CleanVersion returns the version without leading 'v'.
func CleanVersion(v string) string {
	s := strings.TrimSpace(v)
	s = strings.TrimPrefix(s, "v")
	return strings.TrimPrefix(s, "V")
}

// ParseVersion parses a Semantic Version string (with or without leading 'v').
func ParseVersion(s string) (Version, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return Version{}, fmt.Errorf("empty version string")
	}

	trimmed := strings.TrimPrefix(raw, "v")
	trimmed = strings.TrimPrefix(trimmed, "V")

	var build string
	if idx := strings.IndexByte(trimmed, '+'); idx != -1 {
		build = trimmed[idx+1:]
		trimmed = trimmed[:idx]
	}

	var prerelease string
	if idx := strings.IndexByte(trimmed, '-'); idx != -1 {
		prerelease = trimmed[idx+1:]
		trimmed = trimmed[:idx]
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("invalid semver: %q", raw)
	}

	major, err := parseVersionNumber(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version in %q: %w", raw, err)
	}

	minor := 0
	if len(parts) > 1 {
		minor, err = parseVersionNumber(parts[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid minor version in %q: %w", raw, err)
		}
	}

	patch := 0
	if len(parts) > 2 {
		patch, err = parseVersionNumber(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version in %q: %w", raw, err)
		}
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Build:      build,
		Raw:        raw,
	}, nil
}

func parseVersionNumber(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty component")
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return 0, fmt.Errorf("contains non-digit %q", r)
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid integer %q", s)
	}
	return n, nil
}

// Compare compares two versions according to SemVer 2.0.0 precedence rules.
// Returns:
//
//	-1 if a < b
//	 0 if a == b
//	 1 if a > b
func Compare(a, b Version) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}

	// When major, minor, and patch are equal, a pre-release version has lower precedence
	// than a normal version.
	if a.Prerelease == "" && b.Prerelease != "" {
		return 1
	}
	if a.Prerelease != "" && b.Prerelease == "" {
		return -1
	}
	if a.Prerelease == "" && b.Prerelease == "" {
		return 0
	}

	// Compare pre-release identifiers dot-by-dot
	aParts := strings.Split(a.Prerelease, ".")
	bParts := strings.Split(b.Prerelease, ".")
	minLen := len(aParts)
	if len(bParts) < minLen {
		minLen = len(bParts)
	}

	for i := 0; i < minLen; i++ {
		aNum, aIsNum := parseInteger(aParts[i])
		bNum, bIsNum := parseInteger(bParts[i])

		if aIsNum && bIsNum {
			if aNum != bNum {
				if aNum < bNum {
					return -1
				}
				return 1
			}
		} else if aIsNum && !bIsNum {
			// Numeric identifiers have lower precedence than non-numeric identifiers
			return -1
		} else if !aIsNum && bIsNum {
			return 1
		} else {
			// Compare lexical ASCII
			if aParts[i] != bParts[i] {
				if aParts[i] < bParts[i] {
					return -1
				}
				return 1
			}
		}
	}

	if len(aParts) < len(bParts) {
		return -1
	}
	if len(aParts) > len(bParts) {
		return 1
	}
	return 0
}

func parseInteger(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

// IsNewer reports whether target is considered newer than current.
// If current is a development build, any valid release is considered newer.
func IsNewer(current, target string) bool {
	if IsDev(current) {
		return !IsDev(target)
	}
	cVer, err := ParseVersion(current)
	if err != nil {
		return true
	}
	tVer, err := ParseVersion(target)
	if err != nil {
		return false
	}
	return Compare(tVer, cVer) > 0
}

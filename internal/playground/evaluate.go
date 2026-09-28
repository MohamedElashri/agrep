// Package playground prepares normalization examples for the WebAssembly UI.
package playground

import (
	"fmt"
	"unicode/utf8"

	"github.com/MohamedElashri/agrep/arabic"
	"golang.org/x/text/unicode/norm"
)

// Result describes a normalization example. A rule is listed only when
// disabling that rule alone changes the final comparison key.
type Result struct {
	Key   string   `json:"key"`
	Rules []string `json:"rules"`
}

// Evaluate normalizes input using one built-in profile and language selection.
func Evaluate(input, profileName, languageNames string) (Result, error) {
	if !utf8.ValidString(input) {
		return Result{}, fmt.Errorf("input must be valid UTF-8")
	}
	if len(input) > 32*1024 {
		return Result{}, fmt.Errorf("input must be at most 32 KiB")
	}
	var profile arabic.Profile
	switch profileName {
	case "search":
		profile = arabic.ProfileSearch
	case "strict":
		profile = arabic.ProfileStrict
	case "loose":
		profile = arabic.ProfileLoose
	case "lucene":
		profile = arabic.ProfileLucene
	case "camel":
		profile = arabic.ProfileCAMeL
	default:
		return Result{}, fmt.Errorf("unknown profile %q", profileName)
	}
	languages, err := arabic.ParseLanguages(languageNames)
	if err != nil {
		return Result{}, err
	}
	profile.Languages = languages
	key := profile.Normalize(input)
	result := Result{Key: key}
	if norm.NFD.String(input) != input {
		result.Rules = append(result.Rules, "Unicode NFD")
	}
	checks := []struct {
		name    string
		enabled bool
		disable func(*arabic.Profile)
	}{
		{"presentation forms", profile.FoldPresentation, func(p *arabic.Profile) { p.FoldPresentation = false }},
		{"tashkil", profile.StripTashkil, func(p *arabic.Profile) { p.StripTashkil = false }},
		{"tatweel", profile.StripTatweel, func(p *arabic.Profile) { p.StripTatweel = false }},
		{"alef hamza", profile.FoldAlefHamza, func(p *arabic.Profile) { p.FoldAlefHamza = false }},
		{"alef wasla", profile.FoldAlefWasla, func(p *arabic.Profile) { p.FoldAlefWasla = false }},
		{"hamza seat", profile.FoldHamzaSeat, func(p *arabic.Profile) { p.FoldHamzaSeat = false }},
		{"ta marbuta", profile.FoldTaMarbuta, func(p *arabic.Profile) { p.FoldTaMarbuta = false }},
		{"alef maksura", profile.FoldAlefMaksura, func(p *arabic.Profile) { p.FoldAlefMaksura = false }},
		{"joiners", profile.StripJoiners, func(p *arabic.Profile) { p.StripJoiners = false }},
		{"bidi controls", profile.StripBidi, func(p *arabic.Profile) { p.StripBidi = false }},
		{"digits", profile.FoldDigits, func(p *arabic.Profile) { p.FoldDigits = false }},
		{"punctuation", profile.FoldPunctuation, func(p *arabic.Profile) { p.FoldPunctuation = false }},
		{"Quranic marks", profile.StripQuranic, func(p *arabic.Profile) { p.StripQuranic = false }},
		{"rasm", profile.Rasm, func(p *arabic.Profile) { p.Rasm = false }},
		{"language equivalences", profile.Languages != 0, func(p *arabic.Profile) { p.Languages = 0 }},
	}
	for _, check := range checks {
		if !check.enabled {
			continue
		}
		without := profile
		check.disable(&without)
		if without.Normalize(input) != key {
			result.Rules = append(result.Rules, check.name)
		}
	}
	return result, nil
}

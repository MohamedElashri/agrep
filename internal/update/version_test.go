package update

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input      string
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPre    string
		wantBuild  string
		shouldFail bool
	}{
		{"v1.2.3", 1, 2, 3, "", "", false},
		{"1.2.3", 1, 2, 3, "", "", false},
		{"v0.1.0", 0, 1, 0, "", "", false},
		{"v0.1", 0, 1, 0, "", "", false},
		{"v2", 2, 0, 0, "", "", false},
		{"v1.2.3-rc.1", 1, 2, 3, "rc.1", "", false},
		{"v1.2.3-beta.2+build.42", 1, 2, 3, "beta.2", "build.42", false},
		{"v1.2.3+sha123", 1, 2, 3, "", "sha123", false},
		{"", 0, 0, 0, "", "", true},
		{"invalid", 0, 0, 0, "", "", true},
		{"v1.2.3.4", 0, 0, 0, "", "", true},
		{"v-1.0.0", 0, 0, 0, "", "", true},
	}

	for _, tc := range tests {
		v, err := ParseVersion(tc.input)
		if tc.shouldFail {
			if err == nil {
				t.Errorf("ParseVersion(%q) expected error, got nil", tc.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tc.input, err)
			continue
		}
		if v.Major != tc.wantMajor || v.Minor != tc.wantMinor || v.Patch != tc.wantPatch ||
			v.Prerelease != tc.wantPre || v.Build != tc.wantBuild {
			t.Errorf("ParseVersion(%q) = %+v, want major=%d minor=%d patch=%d pre=%q build=%q",
				tc.input, v, tc.wantMajor, tc.wantMinor, tc.wantPatch, tc.wantPre, tc.wantBuild)
		}
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.0.0", "v1.0.0-rc.1", 1},
		{"v1.0.0-rc.1", "v1.0.0", -1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
		{"v1.0.0-beta.1", "v1.0.0-beta.2", -1},
		{"v1.0.0-beta.11", "v1.0.0-beta.2", 1},
		{"v1.0.0-rc.1", "v1.0.0-rc.1", 0},
		{"v1.0.0-alpha", "v1.0.0-alpha.1", -1},
	}

	for _, tc := range tests {
		va, err := ParseVersion(tc.a)
		if err != nil {
			t.Fatalf("ParseVersion(%q) failed: %v", tc.a, err)
		}
		vb, err := ParseVersion(tc.b)
		if err != nil {
			t.Fatalf("ParseVersion(%q) failed: %v", tc.b, err)
		}
		got := Compare(va, vb)
		if got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIsDev(t *testing.T) {
	devs := []string{"dev", "DEV", "(devel)", "", "dev-123", "v1.0.0-dev"}
	nonDevs := []string{"v1.0.0", "0.1.0", "v0.1.0-rc.1"}

	for _, s := range devs {
		if !IsDev(s) {
			t.Errorf("IsDev(%q) should be true", s)
		}
	}
	for _, s := range nonDevs {
		if IsDev(s) {
			t.Errorf("IsDev(%q) should be false", s)
		}
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		current string
		target  string
		want    bool
	}{
		{"dev", "v0.1.0", true},
		{"dev", "dev", false},
		{"v0.1.0", "v0.2.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.2.0", "v0.1.0", false},
		{"0.1.0", "v0.2.0", true},
		{"v1.0.0-rc.1", "v1.0.0", true},
	}

	for _, tc := range tests {
		got := IsNewer(tc.current, tc.target)
		if got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.current, tc.target, got, tc.want)
		}
	}
}

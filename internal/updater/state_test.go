package updater

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		v1       string
		v2       string
		expected int
	}{
		// Equal versions
		{
			name:     "equal versions",
			v1:       "1.2.3",
			v2:       "1.2.3",
			expected: 0,
		},
		{
			name:     "equal versions with v prefix",
			v1:       "v1.2.3",
			v2:       "v1.2.3",
			expected: 0,
		},
		{
			name:     "equal versions mixed prefix",
			v1:       "v1.2.3",
			v2:       "1.2.3",
			expected: 0,
		},

		// v1 > v2 (returns 1)
		{
			name:     "major version greater",
			v1:       "2.0.0",
			v2:       "1.0.0",
			expected: 1,
		},
		{
			name:     "minor version greater",
			v1:       "1.5.0",
			v2:       "1.3.0",
			expected: 1,
		},
		{
			name:     "patch version greater",
			v1:       "1.2.5",
			v2:       "1.2.3",
			expected: 1,
		},
		{
			name:     "double digit patch greater",
			v1:       "0.0.10",
			v2:       "0.0.9",
			expected: 1,
		},
		{
			name:     "double digit minor greater",
			v1:       "1.10.0",
			v2:       "1.9.0",
			expected: 1,
		},
		{
			name:     "complex version greater",
			v1:       "2.10.15",
			v2:       "2.10.9",
			expected: 1,
		},

		// v1 < v2 (returns -1)
		{
			name:     "major version less",
			v1:       "1.0.0",
			v2:       "2.0.0",
			expected: -1,
		},
		{
			name:     "minor version less",
			v1:       "1.3.0",
			v2:       "1.5.0",
			expected: -1,
		},
		{
			name:     "patch version less",
			v1:       "1.2.3",
			v2:       "1.2.5",
			expected: -1,
		},
		{
			name:     "double digit patch less",
			v1:       "0.0.9",
			v2:       "0.0.10",
			expected: -1,
		},
		{
			name:     "double digit minor less",
			v1:       "1.9.0",
			v2:       "1.10.0",
			expected: -1,
		},

		// Prerelease versions
		{
			name:     "prerelease vs release",
			v1:       "1.2.3-beta",
			v2:       "1.2.3",
			expected: 0, // Main version parts are equal (prerelease suffix ignored)
		},
		{
			name:     "prerelease versions",
			v1:       "1.2.4-beta",
			v2:       "1.2.3-alpha",
			expected: 1, // 1.2.4 > 1.2.3
		},

		// Edge cases
		{
			name:     "missing patch in v2",
			v1:       "1.2.3",
			v2:       "1.2",
			expected: 1, // 1.2.3 > 1.2.0
		},
		{
			name:     "missing patch in v1",
			v1:       "1.2",
			v2:       "1.2.3",
			expected: -1, // 1.2.0 < 1.2.3
		},
		{
			name:     "three digit version",
			v1:       "1.100.200",
			v2:       "1.99.300",
			expected: 1, // 100 > 99
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CompareVersions(tt.v1, tt.v2)
			if result != tt.expected {
				t.Errorf("CompareVersions(%q, %q) = %d; expected %d", tt.v1, tt.v2, result, tt.expected)
			}
		})
	}
}

func TestSplitVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected []int
	}{
		{
			name:     "simple version",
			version:  "1.2.3",
			expected: []int{1, 2, 3},
		},
		{
			name:     "double digit version",
			version:  "1.10.25",
			expected: []int{1, 10, 25},
		},
		{
			name:     "triple digit version",
			version:  "100.200.300",
			expected: []int{100, 200, 300},
		},
		{
			name:     "version with prerelease",
			version:  "1.2.3-beta",
			expected: []int{1, 2, 3},
		},
		{
			name:     "version with prerelease and build",
			version:  "1.2.3-beta.1+build.123",
			expected: []int{1, 2, 3},
		},
		{
			name:     "two part version",
			version:  "1.2",
			expected: []int{1, 2},
		},
		{
			name:     "single part version",
			version:  "5",
			expected: []int{5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitVersion(tt.version)
			if len(result) != len(tt.expected) {
				t.Errorf("splitVersion(%q) length = %d; expected %d", tt.version, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("splitVersion(%q)[%d] = %d; expected %d", tt.version, i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestHasUpdate(t *testing.T) {
	tests := []struct {
		name           string
		latestVersion  string
		currentVersion string
		expected       bool
	}{
		{
			name:           "update available",
			latestVersion:  "v0.0.10",
			currentVersion: "v0.0.9",
			expected:       true,
		},
		{
			name:           "no update - same version",
			latestVersion:  "v1.2.3",
			currentVersion: "v1.2.3",
			expected:       false,
		},
		{
			name:           "no update - current is newer",
			latestVersion:  "v1.2.3",
			currentVersion: "v1.2.5",
			expected:       false,
		},
		{
			name:           "update available - major version",
			latestVersion:  "v2.0.0",
			currentVersion: "v1.9.9",
			expected:       true,
		},
		{
			name:           "no latest version set",
			latestVersion:  "",
			currentVersion: "v1.0.0",
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := &UpdateState{
				LatestVersion: tt.latestVersion,
			}
			result := state.HasUpdate(tt.currentVersion)
			if result != tt.expected {
				t.Errorf("HasUpdate(%q) with latest %q = %v; expected %v", tt.currentVersion, tt.latestVersion, result, tt.expected)
			}
		})
	}
}

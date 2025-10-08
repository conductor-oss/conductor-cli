package updater

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	configDirName      = ".conductor-cli"
	updateStateFile    = "update-check.json"
	checkInterval      = 24 * time.Hour
)

// UpdateState represents the state of update checks
type UpdateState struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version,omitempty"`
}

// GetConfigDir returns the path to the CLI config directory (~/.conductor-cli)
func GetConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, configDirName), nil
}

// GetConfigFile returns the path to the main config file
func GetConfigFile() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "config.yaml"), nil
}

// GetUpdateStateFile returns the path to the update state file
func GetUpdateStateFile() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, updateStateFile), nil
}

// LoadState loads the update state from disk
func LoadState() (*UpdateState, error) {
	stateFile, err := GetUpdateStateFile()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty state if file doesn't exist
			return &UpdateState{}, nil
		}
		return nil, err
	}

	var state UpdateState
	if err := json.Unmarshal(data, &state); err != nil {
		// If corrupted, return empty state
		return &UpdateState{}, nil
	}

	return &state, nil
}

// Save saves the update state to disk
func (s *UpdateState) Save() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	stateFile, err := GetUpdateStateFile()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(stateFile, data, 0644)
}

// ShouldCheck returns true if enough time has passed since the last check
func (s *UpdateState) ShouldCheck() bool {
	if s.LastCheck.IsZero() {
		return true
	}
	return time.Since(s.LastCheck) >= checkInterval
}

// HasUpdate returns true if a newer version is available
func (s *UpdateState) HasUpdate(currentVersion string) bool {
	if s.LatestVersion == "" {
		return false
	}
	return CompareVersions(s.LatestVersion, currentVersion) > 0
}

// CompareVersions compares two version strings (semver comparison)
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = trimPrefix(v1, "v")
	v2 = trimPrefix(v2, "v")

	if v1 == v2 {
		return 0
	}

	// Split by '.' and '-' for prerelease versions
	parts1 := splitVersion(v1)
	parts2 := splitVersion(v2)

	// Compare each numeric part
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var p1, p2 int

		if i < len(parts1) {
			p1 = parts1[i]
		}
		if i < len(parts2) {
			p2 = parts2[i]
		}

		if p1 > p2 {
			return 1
		}
		if p1 < p2 {
			return -1
		}
	}

	return 0
}

func splitVersion(v string) []int {
	// Split by '-' to handle prerelease (e.g., "1.2.3-beta")
	mainPart := v
	if idx := indexOf(v, "-"); idx >= 0 {
		mainPart = v[:idx]
	}

	// Split by '.'
	parts := []int{}
	current := 0
	hasNum := false

	for i := 0; i < len(mainPart); i++ {
		if mainPart[i] >= '0' && mainPart[i] <= '9' {
			current = current*10 + int(mainPart[i]-'0')
			hasNum = true
		} else if mainPart[i] == '.' {
			if hasNum {
				parts = append(parts, current)
			}
			current = 0
			hasNum = false
		}
	}
	if hasNum {
		parts = append(parts, current)
	}

	return parts
}

func trimPrefix(s, prefix string) string {
	if len(s) > 0 && s[0:1] == prefix {
		return s[1:]
	}
	return s
}

func indexOf(s, ch string) int {
	for i := 0; i < len(s); i++ {
		if s[i:i+1] == ch {
			return i
		}
	}
	return -1
}

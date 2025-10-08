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

// CompareVersions compares two version strings (simple semver comparison)
// Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
func CompareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = trimPrefix(v1, "v")
	v2 = trimPrefix(v2, "v")

	// Simple comparison for now (can be enhanced with proper semver library)
	if v1 == v2 {
		return 0
	}
	if v1 > v2 {
		return 1
	}
	return -1
}

func trimPrefix(s, prefix string) string {
	if len(s) > 0 && s[0:1] == prefix {
		return s[1:]
	}
	return s
}

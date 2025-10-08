package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

const (
	githubRepo        = "conductor-oss/conductor-cli"
	githubAPITimeout  = 10 * time.Second
	releasesAPIURL    = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	LatestVersion string
	CurrentVersion string
	DownloadURL   string
	ReleaseURL    string
}

// GitHubRelease represents the GitHub API response for a release
type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
	HTMLURL string `json:"html_url"`
}

// CheckForUpdate checks GitHub for the latest release
func CheckForUpdate(ctx context.Context, currentVersion string) (*UpdateInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, githubAPITimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", releasesAPIURL, nil)
	if err != nil {
		return nil, err
	}

	// Set User-Agent header (GitHub API requirement)
	req.Header.Set("User-Agent", "conductor-cli")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var release GitHubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}

	// Find the appropriate asset for current platform
	assetName := getAssetName()
	downloadURL := ""
	for _, asset := range release.Assets {
		if asset.Name == assetName {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	return &UpdateInfo{
		LatestVersion:  release.TagName,
		CurrentVersion: currentVersion,
		DownloadURL:    downloadURL,
		ReleaseURL:     release.HTMLURL,
	}, nil
}

// CheckInBackground performs a background check and updates the state file
func CheckInBackground(ctx context.Context, currentVersion string) {
	go func() {
		state, err := LoadState()
		if err != nil {
			// Silently fail - don't block CLI
			return
		}

		if !state.ShouldCheck() {
			return
		}

		updateInfo, err := CheckForUpdate(ctx, currentVersion)
		if err != nil {
			// Silently fail - don't block CLI or spam errors
			// Just update the timestamp so we don't retry immediately
			state.LastCheck = time.Now()
			_ = state.Save()
			return
		}

		// Update state
		state.LastCheck = time.Now()
		state.LatestVersion = updateInfo.LatestVersion
		_ = state.Save()
	}()
}

// ShouldNotifyUpdate checks if we should notify the user about an update
func ShouldNotifyUpdate(currentVersion string) (bool, string) {
	state, err := LoadState()
	if err != nil {
		return false, ""
	}

	if state.HasUpdate(currentVersion) {
		return true, state.LatestVersion
	}

	return false, ""
}

// getAssetName returns the expected asset name for the current platform
func getAssetName() string {
	// Format: orkes_<os>_<arch> or orkes_<os>_<arch>.exe for windows
	osName := runtime.GOOS
	archName := runtime.GOARCH

	// Map Go arch names to common naming conventions
	switch archName {
	case "amd64":
		archName = "x86_64"
	case "arm64":
		archName = "arm64"
	}

	assetName := fmt.Sprintf("orkes_%s_%s", osName, archName)

	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	return assetName
}

// DownloadBinary downloads the binary from the given URL
func DownloadBinary(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

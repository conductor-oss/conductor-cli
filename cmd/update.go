/*
 * Copyright 2026 Conductor Authors.
 * <p>
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 * <p>
 * http://www.apache.org/licenses/LICENSE-2.0
 * <p>
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */


package cmd

import (
	"bytes"
	"fmt"
	"os"
	"runtime"

	goupdater "github.com/inconshreveable/go-update"
	"github.com/orkes-io/conductor-cli/internal/updater"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:     "update",
	Short:   "Update the CLI to the latest version",
	Long:    "Download and install the latest version of the Conductor CLI from GitHub releases.",
	GroupID: "config",
	RunE:    runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	fmt.Println("Checking for updates...")

	// Check for latest version
	updateInfo, err := updater.CheckForUpdate(ctx, Version)
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	// Compare versions
	if updater.CompareVersions(updateInfo.LatestVersion, Version) <= 0 {
		fmt.Printf("✓ Already on the latest version: %s\n", Version)
		return nil
	}

	fmt.Printf("Update available: %s → %s\n", Version, updateInfo.LatestVersion)

	// Check if we have a download URL for this platform
	if updateInfo.DownloadURL == "" {
		fmt.Printf("\nNo pre-built binary available for %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Please download manually from: %s\n", updateInfo.ReleaseURL)
		return nil
	}

	fmt.Printf("Downloading from: %s\n", updateInfo.DownloadURL)

	// Download the binary
	binaryData, err := updater.DownloadBinary(ctx, updateInfo.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}

	fmt.Printf("Downloaded %d bytes\n", len(binaryData))

	// Apply the update (replace current binary)
	fmt.Println("Applying update...")
	err = goupdater.Apply(bytes.NewReader(binaryData), goupdater.Options{})
	if err != nil {
		// Check if it's a permissions error
		if os.IsPermission(err) {
			return fmt.Errorf("permission denied: try running with elevated privileges (sudo)")
		}
		return fmt.Errorf("failed to apply update: %w", err)
	}

	// Update the state file with the new version
	state := &updater.UpdateState{
		LatestVersion: updateInfo.LatestVersion,
	}
	if err := state.Save(); err != nil {
		log.Warnf("Failed to update state file: %v", err)
	}

	fmt.Printf("✓ Successfully updated to %s\n", updateInfo.LatestVersion)
	fmt.Println("\nPlease restart your terminal or run the command again to use the new version.")

	return nil
}

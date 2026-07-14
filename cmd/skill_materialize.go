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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/conductor-oss/conductor-cli/internal/skill"
	"github.com/conductor-oss/conductor-cli/internal/updater"
)

// skillCacheDirName is the sub-directory of the Conductor CLI config home under
// which downloaded skill packages are cached (config-driven, never ~/.agentspan).
const skillCacheDirName = "skills"

var cacheSegmentUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// materializeSkill resolves a skill argument to a local directory. A local skill
// directory is used as-is; otherwise the argument is treated as a registered skill
// name, downloaded (and cached), and its cache directory returned. detail is nil
// for a local directory.
func materializeSkill(ctx context.Context, svc skill.Service, skillArg, version string) (dir string, detail *skill.Detail, err error) {
	if isSkillDirectory(skillArg) {
		return skillArg, nil, nil
	}
	d, err := svc.Get(ctx, skillArg, version)
	if err != nil {
		return "", nil, fmt.Errorf("skill %q is not a local skill directory and was not found on the server: %w", skillArg, err)
	}
	dir, err = ensureCachedSkillPackage(ctx, svc, d)
	if err != nil {
		return "", nil, err
	}
	return dir, &d, nil
}

// ensureCachedSkillPackage returns the local files directory for a registered
// skill, downloading and extracting the package when the cache is absent or stale.
// The download is verified against the server checksum and installed atomically.
func ensureCachedSkillPackage(ctx context.Context, svc skill.Service, detail skill.Detail) (string, error) {
	cacheDir, filesDir, checksumPath, err := skillCachePaths(detail)
	if err != nil {
		return "", err
	}
	if isCachedSkillCurrent(filesDir, checksumPath, detail.Checksum) {
		return filesDir, nil
	}

	data, err := svc.DownloadPackage(ctx, detail.Name, detail.Version)
	if err != nil {
		return "", err
	}
	if checksum := strings.TrimSpace(detail.Checksum); checksum != "" {
		if actual := skillPackageChecksum(data); !strings.EqualFold(actual, checksum) {
			return "", fmt.Errorf("downloaded skill package checksum mismatch for %s@%s: expected %s, got %s",
				detail.Name, detail.Version, checksum, actual)
		}
	}

	parentDir := filepath.Dir(cacheDir)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		return "", fmt.Errorf("create skill cache parent: %w", err)
	}
	tmpDir, err := os.MkdirTemp(parentDir, "."+filepath.Base(cacheDir)+"-*")
	if err != nil {
		return "", fmt.Errorf("create temp skill cache: %w", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := extractSkillPackage(data, filepath.Join(tmpDir, "files")); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "checksum"), []byte(detail.Checksum), 0o600); err != nil {
		return "", fmt.Errorf("write skill cache checksum: %w", err)
	}
	if err := os.RemoveAll(cacheDir); err != nil {
		return "", fmt.Errorf("clear stale skill cache: %w", err)
	}
	if err := os.Rename(tmpDir, cacheDir); err != nil {
		return "", fmt.Errorf("install skill cache: %w", err)
	}
	cleanupTmp = false
	return filesDir, nil
}

// skillCachePaths derives the cache directory for a skill under the Conductor CLI
// config home (config-driven, not ~/.agentspan).
func skillCachePaths(detail skill.Detail) (cacheDir, filesDir, checksumPath string, err error) {
	configDir, err := updater.GetConfigDir()
	if err != nil {
		return "", "", "", fmt.Errorf("resolve config directory: %w", err)
	}
	cacheDir = filepath.Join(configDir, skillCacheDirName, safeCacheSegment(detail.Name), safeCacheSegment(detail.Version))
	return cacheDir, filepath.Join(cacheDir, "files"), filepath.Join(cacheDir, "checksum"), nil
}

// safeCacheSegment makes a name/version safe as a single path segment, disambiguating
// with a content hash suffix when characters had to be replaced.
func safeCacheSegment(value string) string {
	cleaned := strings.Trim(cacheSegmentUnsafe.ReplaceAllString(value, "_"), "._-")
	if cleaned == "" {
		cleaned = "unnamed"
	}
	if cleaned != value {
		sum := sha256.Sum256([]byte(value))
		cleaned = cleaned + "-" + hex.EncodeToString(sum[:])[:8]
	}
	return cleaned
}

func skillPackageChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// isCachedSkillCurrent reports whether a cached skill's files exist and match the
// expected checksum.
func isCachedSkillCurrent(filesDir, checksumPath, checksum string) bool {
	if !isSkillDirectory(filesDir) {
		return false
	}
	if checksum == "" {
		return true
	}
	data, err := os.ReadFile(checksumPath)
	return err == nil && strings.TrimSpace(string(data)) == checksum
}

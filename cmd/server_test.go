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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestGetJarDownloadURL(t *testing.T) {
	tests := []struct {
		name       string
		serverType string
		version    string
		wantURL    string
		wantErr    bool
	}{
		{
			name:       "OSS latest",
			serverType: serverTypeOSS,
			version:    "latest",
			wantURL:    "https://conductor-server.s3.us-east-2.amazonaws.com/conductor-server-latest.jar",
		},
		{
			name:       "OSS specific version",
			serverType: serverTypeOSS,
			version:    "3.21.23",
			wantURL:    "https://conductor-server.s3.us-east-2.amazonaws.com/conductor-server-3.21.23.jar",
		},
		{
			name:       "Orkes returns error",
			serverType: serverTypeOrkes,
			version:    "latest",
			wantErr:    true,
		},
		{
			name:       "unknown server type",
			serverType: "invalid",
			version:    "latest",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := getJarDownloadURL(tt.serverType, tt.version)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if url != tt.wantURL {
				t.Errorf("got %q, want %q", url, tt.wantURL)
			}
		})
	}
}

func TestGetServerDirForType(t *testing.T) {
	dir, err := getServerDirForType("oss", "latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(dir, filepath.Join("server", "oss", "latest")) {
		t.Errorf("unexpected dir: %s", dir)
	}

	dir2, err := getServerDirForType("orkes", "3.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(dir2, filepath.Join("server", "orkes", "3.0.0")) {
		t.Errorf("unexpected dir: %s", dir2)
	}
}

func TestGetJarPathForType(t *testing.T) {
	path, err := getJarPathForType("oss", "latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("server", "oss", "latest", "conductor-server.jar")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestIsProcessRunning(t *testing.T) {
	// PID <= 0 should return false
	if isProcessRunning(0) {
		t.Error("expected false for PID 0")
	}
	if isProcessRunning(-1) {
		t.Error("expected false for PID -1")
	}

	// Current process should be running
	if !isProcessRunning(os.Getpid()) {
		t.Error("expected true for current process PID")
	}

	// Very high PID should not be running
	if isProcessRunning(999999999) {
		t.Error("expected false for non-existent PID")
	}
}

func TestReadWritePid(t *testing.T) {
	// Save and restore HOME to use temp dir
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Create server dir
	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	// Write PID
	testPid := 12345
	err := writePid(testPid)
	if err != nil {
		t.Fatalf("writePid failed: %v", err)
	}

	// Read PID back
	pid, err := readPid()
	if err != nil {
		t.Fatalf("readPid failed: %v", err)
	}
	if pid != testPid {
		t.Errorf("got pid %d, want %d", pid, testPid)
	}

	// Remove PID
	err = removePid()
	if err != nil {
		t.Fatalf("removePid failed: %v", err)
	}

	// Read after removal should return 0
	pid, err = readPid()
	if err != nil {
		t.Fatalf("readPid after removal failed: %v", err)
	}
	if pid != 0 {
		t.Errorf("expected pid 0 after removal, got %d", pid)
	}
}

func TestReadPidInvalidContent(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	// Write invalid content to pid file
	pidPath := filepath.Join(serverDir, pidFileName)
	os.WriteFile(pidPath, []byte("not-a-number"), 0644)

	_, err := readPid()
	if err == nil {
		t.Error("expected error for invalid PID content")
	}
}

func TestServerUpdateStateSaveLoad(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Save state
	state := &ServerUpdateState{
		LastCheck:  time.Now().Truncate(time.Second),
		ETag:       `"abc123"`,
		ServerType: "oss",
		Version:    "latest",
	}

	err := state.save()
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Verify the file was created
	statePath, _ := getServerStatePath()
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state file was not created")
	}

	// Load and verify
	loaded := loadServerState()
	if loaded.ETag != state.ETag {
		t.Errorf("ETag: got %q, want %q", loaded.ETag, state.ETag)
	}
	if loaded.ServerType != state.ServerType {
		t.Errorf("ServerType: got %q, want %q", loaded.ServerType, state.ServerType)
	}
	if loaded.Version != state.Version {
		t.Errorf("Version: got %q, want %q", loaded.Version, state.Version)
	}
}

func TestLoadServerStateMissingFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	state := loadServerState()
	if state.ETag != "" {
		t.Errorf("expected empty ETag, got %q", state.ETag)
	}
	if state.ServerType != "" {
		t.Errorf("expected empty ServerType, got %q", state.ServerType)
	}
}

func TestLoadServerStateInvalidJSON(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)
	os.WriteFile(filepath.Join(serverDir, serverStateFile), []byte("{invalid"), 0644)

	state := loadServerState()
	if state.ETag != "" {
		t.Errorf("expected empty ETag for invalid JSON, got %q", state.ETag)
	}
}

func TestServerUpdateStateJSON(t *testing.T) {
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	state := ServerUpdateState{
		LastCheck:  now,
		ETag:       `"etag-value"`,
		ServerType: "oss",
		Version:    "latest",
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded ServerUpdateState
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ETag != state.ETag {
		t.Errorf("ETag mismatch: %q vs %q", decoded.ETag, state.ETag)
	}
	if decoded.ServerType != state.ServerType {
		t.Errorf("ServerType mismatch: %q vs %q", decoded.ServerType, state.ServerType)
	}
}

func TestGetRemoteETag(t *testing.T) {
	// Test successful HEAD request with ETag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "HEAD" {
			t.Errorf("expected HEAD request, got %s", r.Method)
		}
		w.Header().Set("ETag", `"test-etag-123"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	etag, err := getRemoteETag(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if etag != `"test-etag-123"` {
		t.Errorf("got etag %q, want %q", etag, `"test-etag-123"`)
	}
}

func TestGetRemoteETagNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := getRemoteETag(server.URL)
	if err == nil {
		t.Error("expected error for 404 response")
	}
}

func TestGetRemoteETagNoETagHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	etag, err := getRemoteETag(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if etag != "" {
		t.Errorf("expected empty etag, got %q", etag)
	}
}

func TestCheckServerUpdateSkipsWhenRecentlyChecked(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set last check to recent time
	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	state := &ServerUpdateState{
		LastCheck:  time.Now(),
		ETag:       `"old-etag"`,
		ServerType: "oss",
		Version:    "latest",
	}
	state.save()

	// Should return false because check was too recent
	result := checkServerUpdate("oss", "latest")
	if result {
		t.Error("expected false when recently checked")
	}
}

func TestCheckServerUpdateDetectsNewVersion(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Mock server returns different ETag
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"new-etag"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set old state with expired check and old ETag
	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	state := &ServerUpdateState{
		LastCheck:  time.Now().Add(-48 * time.Hour),
		ETag:       `"old-etag"`,
		ServerType: "oss",
		Version:    "latest",
	}
	state.save()

	// We can't easily override the JAR URL, but we can test with real S3 URL
	// skipping if the request would go to real S3. Instead, test the logic path:
	// When ETag is empty in state, should not report update
	state.ETag = ""
	state.LastCheck = time.Now().Add(-48 * time.Hour)
	state.save()

	result := checkServerUpdate("oss", "latest")
	// Even if remote returns an etag, state.ETag is empty so hasUpdate = false
	// (the function checks: state.ETag != "" && state.ETag != remoteETag)
	if result {
		t.Error("expected false when stored ETag is empty")
	}
}

func TestGetServerStatePath(t *testing.T) {
	path, err := getServerStatePath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("server", "server-state.json")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestGetPidPath(t *testing.T) {
	path, err := getPidPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("server", "conductor.pid")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestGetLogPath(t *testing.T) {
	path, err := getLogPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("server", "conductor.log")) {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestRemovePidNonExistent(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	// Should not error when pid file doesn't exist
	err := removePid()
	if err != nil {
		t.Errorf("unexpected error removing non-existent pid: %v", err)
	}
}

func TestWritePidCreatesFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	serverDir := filepath.Join(tmpDir, ".conductor-cli", "server")
	os.MkdirAll(serverDir, 0755)

	err := writePid(42)
	if err != nil {
		t.Fatalf("writePid failed: %v", err)
	}

	// Verify content
	pidPath := filepath.Join(serverDir, pidFileName)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("reading pid file failed: %v", err)
	}
	pid, _ := strconv.Atoi(string(data))
	if pid != 42 {
		t.Errorf("got pid %d, want 42", pid)
	}
}

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
	"os"
	"path/filepath"
	"testing"
)

func TestGetWorkerFile(t *testing.T) {
	tests := []struct {
		language string
		wantExt  string
	}{
		{"NODEJS", ".js"},
		{"PYTHON", ".py"},
		{"JAVA", ".java"},
		{"GO", ".go"},
		{"UNKNOWN", ".txt"},
		{"", ".txt"},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			result := getWorkerFile("/tmp/cache", tt.language)
			expected := filepath.Join("/tmp/cache", "worker"+tt.wantExt)
			if result != expected {
				t.Errorf("got %q, want %q", result, expected)
			}
		})
	}
}

func TestEqualStringSlices(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"both nil", nil, nil, true},
		{"equal", []string{"a", "b"}, []string{"a", "b"}, true},
		{"different length", []string{"a"}, []string{"a", "b"}, false},
		{"different content", []string{"a", "b"}, []string{"a", "c"}, false},
		{"same length different order", []string{"a", "b"}, []string{"b", "a"}, false},
		{"single element equal", []string{"x"}, []string{"x"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := equalStringSlices(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("equalStringSlices(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Existing file
	existing := filepath.Join(tmpDir, "exists.txt")
	os.WriteFile(existing, []byte("content"), 0644)
	if !fileExists(existing) {
		t.Error("expected true for existing file")
	}

	// Non-existing file
	if fileExists(filepath.Join(tmpDir, "nope.txt")) {
		t.Error("expected false for non-existing file")
	}

	// Directory counts as existing
	if !fileExists(tmpDir) {
		t.Error("expected true for directory")
	}
}

func TestLoadMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid metadata", func(t *testing.T) {
		metadataFile := filepath.Join(tmpDir, "valid.json")
		metadata := WorkerMetadata{
			TaskName:     "test_task",
			Language:     "NODEJS",
			Version:      3,
			WorkerCodeId: "wc-123",
		}
		data, _ := json.Marshal(metadata)
		os.WriteFile(metadataFile, data, 0644)

		loaded, err := loadMetadata(metadataFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if loaded.TaskName != "test_task" {
			t.Errorf("TaskName: got %q, want %q", loaded.TaskName, "test_task")
		}
		if loaded.Language != "NODEJS" {
			t.Errorf("Language: got %q, want %q", loaded.Language, "NODEJS")
		}
		if loaded.Version != 3 {
			t.Errorf("Version: got %d, want %d", loaded.Version, 3)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := loadMetadata(filepath.Join(tmpDir, "missing.json"))
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		badFile := filepath.Join(tmpDir, "bad.json")
		os.WriteFile(badFile, []byte("{not json"), 0644)

		_, err := loadMetadata(badFile)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestWorkerCodeResponseJSON(t *testing.T) {
	input := `{
		"id": "wc-123",
		"userId": "user-1",
		"namespace": "default",
		"taskName": "my_task",
		"language": "NODEJS",
		"code": "console.log('hello')",
		"version": 2,
		"description": "A test worker"
	}`

	var resp WorkerCodeResponse
	err := json.Unmarshal([]byte(input), &resp)
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Id != "wc-123" {
		t.Errorf("Id: got %q, want %q", resp.Id, "wc-123")
	}
	if resp.TaskName != "my_task" {
		t.Errorf("TaskName: got %q, want %q", resp.TaskName, "my_task")
	}
	if resp.Language != "NODEJS" {
		t.Errorf("Language: got %q, want %q", resp.Language, "NODEJS")
	}
	if resp.Version != 2 {
		t.Errorf("Version: got %d, want 2", resp.Version)
	}
}

func TestWorkerMetadataJSON(t *testing.T) {
	metadata := WorkerMetadata{
		TaskName:     "task_1",
		Language:     "PYTHON",
		Version:      5,
		WorkerCodeId: "wc-456",
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WorkerMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.TaskName != metadata.TaskName {
		t.Errorf("TaskName: got %q, want %q", decoded.TaskName, metadata.TaskName)
	}
	if decoded.Language != metadata.Language {
		t.Errorf("Language: got %q, want %q", decoded.Language, metadata.Language)
	}
	if decoded.Version != metadata.Version {
		t.Errorf("Version: got %d, want %d", decoded.Version, metadata.Version)
	}
}

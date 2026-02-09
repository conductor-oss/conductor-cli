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
	"testing"
	"time"
)

func TestParseTimeToEpochMillis(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int64
		expectError bool
	}{
		// Test empty string
		{
			name:        "empty string",
			input:       "",
			expected:    0,
			expectError: true,
		},
		// Test YYYY-MM-DD HH:MM:SS format
		{
			name:        "YYYY-MM-DD HH:MM:SS format",
			input:       "2023-12-25 15:30:45",
			expected:    time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test RFC3339 UTC format
		{
			name:        "RFC3339 UTC format",
			input:       "2023-12-25T15:30:45Z",
			expected:    time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test RFC3339 without timezone
		{
			name:        "RFC3339 without timezone",
			input:       "2023-12-25T15:30:45",
			expected:    time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test YYYY-MM-DD HH:MM format
		{
			name:        "YYYY-MM-DD HH:MM format",
			input:       "2023-12-25 15:30",
			expected:    time.Date(2023, 12, 25, 15, 30, 0, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test YYYY-MM-DD format
		{
			name:        "YYYY-MM-DD format",
			input:       "2023-12-25",
			expected:    time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test MM/DD/YYYY HH:MM:SS format
		{
			name:        "MM/DD/YYYY HH:MM:SS format",
			input:       "12/25/2023 15:30:45",
			expected:    time.Date(2023, 12, 25, 15, 30, 45, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test MM/DD/YYYY format
		{
			name:        "MM/DD/YYYY format",
			input:       "12/25/2023",
			expected:    time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC).Unix() * 1000,
			expectError: false,
		},
		// Test epoch milliseconds input
		{
			name:        "epoch milliseconds",
			input:       "1703520645000", // 2023-12-25 15:30:45 UTC in milliseconds
			expected:    1703520645000,
			expectError: false,
		},
		// Test epoch seconds (should work as milliseconds)
		{
			name:        "epoch seconds as milliseconds",
			input:       "1703520645",
			expected:    1703520645,
			expectError: false,
		},
		// Test invalid format
		{
			name:        "invalid format",
			input:       "invalid-date",
			expected:    0,
			expectError: true,
		},
		// Test malformed date
		{
			name:        "malformed date",
			input:       "2023-13-45",
			expected:    0,
			expectError: true,
		},
		// Test negative epoch
		{
			name:        "negative epoch",
			input:       "-1000",
			expected:    -1000,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimeToEpochMillis(tt.input)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseTimeToEpochMillis_EdgeCases(t *testing.T) {
	// Test leap year
	t.Run("leap year", func(t *testing.T) {
		result, err := parseTimeToEpochMillis("2024-02-29")
		if err != nil {
			t.Errorf("unexpected error for leap year: %v", err)
		}
		expected := time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC).Unix() * 1000
		if result != expected {
			t.Errorf("expected %d, got %d", expected, result)
		}
	})

	// Test year 2000 (Y2K)
	t.Run("year 2000", func(t *testing.T) {
		result, err := parseTimeToEpochMillis("2000-01-01 00:00:00")
		if err != nil {
			t.Errorf("unexpected error for Y2K: %v", err)
		}
		expected := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC).Unix() * 1000
		if result != expected {
			t.Errorf("expected %d, got %d", expected, result)
		}
	})

	// Test very large epoch timestamp
	t.Run("large epoch timestamp", func(t *testing.T) {
		largeTimestamp := "9223372036854775807" // Max int64
		result, err := parseTimeToEpochMillis(largeTimestamp)
		if err != nil {
			t.Errorf("unexpected error for large timestamp: %v", err)
		}
		if result != 9223372036854775807 {
			t.Errorf("expected 9223372036854775807, got %d", result)
		}
	})
}
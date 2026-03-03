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

	"github.com/spf13/cobra"
)

func TestGetOutputFormat(t *testing.T) {
	tests := []struct {
		name       string
		jsonFlag   bool
		csvFlag    bool
		want       OutputFormat
		wantError  bool
	}{
		{
			name:     "no flags returns table",
			jsonFlag: false,
			csvFlag:  false,
			want:     OutputFormatTable,
		},
		{
			name:     "json flag returns JSON",
			jsonFlag: true,
			csvFlag:  false,
			want:     OutputFormatJSON,
		},
		{
			name:     "csv flag returns CSV",
			jsonFlag: false,
			csvFlag:  true,
			want:     OutputFormatCSV,
		},
		{
			name:      "both flags returns error",
			jsonFlag:  true,
			csvFlag:   true,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().Bool("json", tt.jsonFlag, "")
			cmd.Flags().Bool("csv", tt.csvFlag, "")

			got, err := GetOutputFormat(cmd)
			if tt.wantError {
				if err == nil {
					t.Errorf("GetOutputFormat() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetOutputFormat() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("GetOutputFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEscapeCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain string unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "quotes are doubled",
			input: `say "hello"`,
			want:  `say ""hello""`,
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "string with commas unchanged",
			input: "one,two,three",
			want:  "one,two,three",
		},
		{
			name:  "multiple quotes",
			input: `"a" and "b"`,
			want:  `""a"" and ""b""`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeCSV(tt.input)
			if got != tt.want {
				t.Errorf("EscapeCSV(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

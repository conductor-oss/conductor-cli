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
	"strings"
	"testing"
)

func TestIsTokenExpired(t *testing.T) {
	tests := []struct {
		name          string
		expiryUnix    int64
		bufferSeconds int64
		want          bool
		description   string
	}{
		{
			name:          "long-lived token (no expiry)",
			expiryUnix:    -1,
			bufferSeconds: 300,
			want:          false,
			description:   "Tokens with -1 expiry should never be considered expired",
		},
		{
			name:          "unknown expiry (zero)",
			expiryUnix:    0,
			bufferSeconds: 300,
			want:          true,
			description:   "Tokens with 0 expiry should be considered expired",
		},
		{
			name:          "future expiry",
			expiryUnix:    9999999999, // Far future
			bufferSeconds: 300,
			want:          false,
			description:   "Tokens that expire far in the future should not be expired",
		},
		{
			name:          "past expiry",
			expiryUnix:    1000000000, // Jan 2001
			bufferSeconds: 0,
			want:          true,
			description:   "Tokens that expired in the past should be expired",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTokenExpired(tt.expiryUnix, tt.bufferSeconds)
			if got != tt.want {
				t.Errorf("isTokenExpired(%d, %d) = %v, want %v - %s",
					tt.expiryUnix, tt.bufferSeconds, got, tt.want, tt.description)
			}
		})
	}
}

func TestGetTokenExpiry(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		wantError   bool
		description string
	}{
		{
			name:        "token without exp claim",
			token:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantError:   true,
			description: "JWT without exp claim should return error",
		},
		{
			name:        "token with exp claim",
			token:       "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNzAwMDAwMDAwfQ.4Adcj0mI2Z0jVl5fOjmGCKmGWltVtH_JxJ2iJ7k02Bw",
			wantError:   false,
			description: "JWT with exp claim should parse successfully",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getTokenExpiry(tt.token)
			gotError := err != nil
			if gotError != tt.wantError {
				t.Errorf("getTokenExpiry() error = %v, wantError %v - %s",
					err, tt.wantError, tt.description)
			}
		})
	}
}

func TestValidateUserToken(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantError bool
	}{
		{
			name:      "empty token returns error",
			token:     "",
			wantError: true,
		},
		{
			name:      "non-JWT token passes (server will validate)",
			token:     "some-opaque-token-value",
			wantError: false,
		},
		{
			name: "valid JWT with future exp passes",
			// JWT with exp: 9999999999 (far future)
			token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZXhwIjo5OTk5OTk5OTk5fQ.Vx6H4JbpOKdVwi0gC73qfaKMVpkBRXMOxHeE9xDIhQQ",
			wantError: false,
		},
		{
			name: "expired JWT returns error",
			// JWT with exp: 1000000000 (Jan 2001)
			token:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwiZXhwIjoxMDAwMDAwMDAwfQ.Iy63AiTmaZLJEvQBUwxWkFo9EvHdWkJ4JFtVSeNz3LQ",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUserToken(tt.token)
			gotError := err != nil
			if gotError != tt.wantError {
				t.Errorf("validateUserToken(%q) error = %v, wantError %v", tt.token, err, tt.wantError)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name    string
		seconds int64
		want    string
	}{
		{
			name:    "seconds",
			seconds: 45,
			want:    "45 seconds",
		},
		{
			name:    "minutes",
			seconds: 300,
			want:    "5 minutes",
		},
		{
			name:    "hours",
			seconds: 7200,
			want:    "2.0 hours",
		},
		{
			name:    "days",
			seconds: 172800,
			want:    "2.0 days",
		},
		{
			name:    "zero seconds",
			seconds: 0,
			want:    "0 seconds",
		},
		{
			name:    "just under a minute",
			seconds: 59,
			want:    "59 seconds",
		},
		{
			name:    "exactly one minute",
			seconds: 60,
			want:    "1 minutes",
		},
		{
			name:    "just under an hour",
			seconds: 3599,
			want:    "59 minutes",
		},
		{
			name:    "exactly one hour",
			seconds: 3600,
			want:    "1.0 hours",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.seconds)
			if got != tt.want {
				t.Errorf("formatDuration(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestGetTokenExpiryManual(t *testing.T) {
	tests := []struct {
		name      string
		token     string
		wantError bool
		errContains string
	}{
		{
			name:        "invalid format - no dots",
			token:       "notajwt",
			wantError:   true,
			errContains: "invalid JWT format",
		},
		{
			name:        "invalid format - only 2 parts",
			token:       "header.payload",
			wantError:   true,
			errContains: "invalid JWT format",
		},
		{
			name:        "invalid base64 payload",
			token:       "header.!!!invalid!!!.signature",
			wantError:   true,
			errContains: "failed to decode",
		},
		{
			name: "valid JWT with exp claim",
			// base64url of {"sub":"123","exp":1700000000}
			token:     "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMiLCJleHAiOjE3MDAwMDAwMDB9.signature",
			wantError: false,
		},
		{
			name: "valid JWT without exp claim",
			// base64url of {"sub":"123","name":"John"}
			token:     "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjMiLCJuYW1lIjoiSm9obiJ9.signature",
			wantError: true,
			errContains: "does not contain exp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getTokenExpiryManual(tt.token)
			gotError := err != nil
			if gotError != tt.wantError {
				t.Errorf("getTokenExpiryManual() error = %v, wantError %v", err, tt.wantError)
			}
			if tt.wantError && tt.errContains != "" && err != nil {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			}
		})
	}
}

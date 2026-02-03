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

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

// Absorbs the clock ticking between the call and the assertion.
const windowTolerance = int64(2000)

func assertNear(t *testing.T, label string, got, want int64) {
	t.Helper()
	if diff := got - want; diff > windowTolerance || diff < -windowTolerance {
		t.Errorf("%s = %d, want ~%d (diff %d ms)", label, got, want, diff)
	}
}

func TestBuildExecutionWindowSince(t *testing.T) {
	now := time.Now()
	from, to, err := buildExecutionWindow("1h", "")
	if err != nil {
		t.Fatalf("buildExecutionWindow: %v", err)
	}
	assertNear(t, "from", from, now.Add(-time.Hour).UnixMilli())
	if to != 0 {
		t.Errorf("to = %d, want 0 (unbounded)", to)
	}
}

func TestBuildExecutionWindowRelativePrefix(t *testing.T) {
	now := time.Now()
	from, to, err := buildExecutionWindow("", "now-7d")
	if err != nil {
		t.Fatalf("buildExecutionWindow: %v", err)
	}
	assertNear(t, "from", from, now.Add(-7*24*time.Hour).UnixMilli())
	assertNear(t, "to", to, now.UnixMilli())
}

func TestBuildExecutionWindowWithoutPrefix(t *testing.T) {
	now := time.Now()
	from, _, err := buildExecutionWindow("", "30m")
	if err != nil {
		t.Fatalf("buildExecutionWindow: %v", err)
	}
	assertNear(t, "from", from, now.Add(-30*time.Minute).UnixMilli())
}

func TestBuildExecutionWindowBothFlagsIntersect(t *testing.T) {
	now := time.Now()
	from, to, err := buildExecutionWindow("7d", "now-1h")
	if err != nil {
		t.Fatalf("buildExecutionWindow: %v", err)
	}
	assertNear(t, "from", from, now.Add(-time.Hour).UnixMilli())
	assertNear(t, "to", to, now.UnixMilli())
}

func TestBuildExecutionWindowNoFlags(t *testing.T) {
	from, to, err := buildExecutionWindow("", "")
	if err != nil {
		t.Fatalf("buildExecutionWindow: %v", err)
	}
	if from != 0 || to != 0 {
		t.Errorf("from/to = %d/%d, want 0/0", from, to)
	}
}

func TestBuildExecutionWindowInvalidValues(t *testing.T) {
	if _, _, err := buildExecutionWindow("yesterday", ""); err == nil {
		t.Error("--since yesterday: got nil error, want error")
	}
	if _, _, err := buildExecutionWindow("", "now-lately"); err == nil {
		t.Error("--window now-lately: got nil error, want error")
	}
}

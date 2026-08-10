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

package cliconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveMapsDefaultAndNamedProfiles(t *testing.T) {
	dir := "/cfg"
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		// The alias is the point of #98: an empty name and "default" must land
		// on the same file, or config save writes somewhere config list is not
		// looking.
		{"empty name is the default config", "", "/cfg/config.yaml"},
		{"default is an alias for the same file", "default", "/cfg/config.yaml"},
		{"named profile", "prod", "/cfg/config-prod.yaml"},
		{"name containing default is still named", "default-2", "/cfg/config-default-2.yaml"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Resolve(dir, tt.profile); got != tt.want {
				t.Errorf("Resolve(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

func TestFileNameMatchesViperConfigName(t *testing.T) {
	if got := FileName(""); got != "config" {
		t.Errorf(`FileName("") = %q, want "config"`, got)
	}
	if got := FileName("default"); got != "config" {
		t.Errorf(`FileName("default") = %q, want "config"`, got)
	}
	if got := FileName("prod"); got != "config-prod" {
		t.Errorf(`FileName("prod") = %q, want "config-prod"`, got)
	}
}

func TestIsDefault(t *testing.T) {
	for _, p := range []string{"", "default"} {
		if !IsDefault(p) {
			t.Errorf("IsDefault(%q) = false, want true", p)
		}
	}
	for _, p := range []string{"prod", "Default", "default2"} {
		if IsDefault(p) {
			t.Errorf("IsDefault(%q) = true, want false", p)
		}
	}
}

func TestStrayDefaultFile(t *testing.T) {
	dir := t.TempDir()
	if got := StrayDefaultFile(dir); got != "" {
		t.Errorf("StrayDefaultFile on clean dir = %q, want empty", got)
	}

	stray := filepath.Join(dir, "config-default.yaml")
	if err := os.WriteFile(stray, []byte("server: http://x\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := StrayDefaultFile(dir); got != stray {
		t.Errorf("StrayDefaultFile = %q, want %q", got, stray)
	}
}

func TestMaskHidesSecretsButKeepsUnsetDistinguishable(t *testing.T) {
	if got := Mask("auth-token", "abc123"); got != "****" {
		t.Errorf("Mask(auth-token) = %q, want ****", got)
	}
	if got := Mask("auth-secret", "s3cret"); got != "****" {
		t.Errorf("Mask(auth-secret) = %q, want ****", got)
	}
	if got := Mask("server", "http://localhost:8080/api"); got != "http://localhost:8080/api" {
		t.Errorf("Mask(server) masked a non-secret: %q", got)
	}
	// An unset secret must not render as "****", or an empty config looks
	// populated.
	if got := Mask("auth-token", ""); got != "" {
		t.Errorf("Mask(auth-token, empty) = %q, want empty", got)
	}
}

// writeConfig writes a config file and returns its path.
func writeConfig(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSourceOfDefaultProfileMergesEnvOverFile(t *testing.T) {
	dir := t.TempDir()
	file := writeConfig(t, dir, "config.yaml", "server: http://from-file\nauth-token: tok\n")

	t.Setenv("CONDUCTOR_SERVER_URL", "http://from-env")
	os.Unsetenv("CONDUCTOR_AUTH_TOKEN")

	r := NewResolution("", file, nil)

	// This merge is the behaviour users cannot currently see: env wins for the
	// key it sets, and the file still supplies the rest.
	if got := r.SourceOf("server"); got.Kind != SourceEnv || got.Detail != "CONDUCTOR_SERVER_URL" {
		t.Errorf("server source = %+v, want env CONDUCTOR_SERVER_URL", got)
	}
	if got := r.SourceOf("auth-token"); got.Kind != SourceFile || got.Detail != file {
		t.Errorf("auth-token source = %+v, want file %s", got, file)
	}
	if got := r.SourceOf("server-type"); got.Kind != SourceDefault {
		t.Errorf("server-type source = %+v, want default", got)
	}
}

func TestSourceOfNamedProfileIgnoresEnv(t *testing.T) {
	dir := t.TempDir()
	file := writeConfig(t, dir, "config-prod.yaml", "server: http://from-profile\n")

	// The environment is set but not bound: cmd/root.go skips BindEnv when a
	// named profile is active, so reporting env here would name a value that is
	// not the one in use.
	t.Setenv("CONDUCTOR_SERVER_URL", "http://from-env")

	r := NewResolution("prod", file, nil)
	if r.EnvBound {
		t.Error("EnvBound = true for a named profile, want false")
	}
	if got := r.SourceOf("server"); got.Kind != SourceFile || got.Detail != file {
		t.Errorf("server source = %+v, want file %s", got, file)
	}
}

func TestSourceOfDefaultAliasBehavesLikeNoProfile(t *testing.T) {
	dir := t.TempDir()
	file := writeConfig(t, dir, "config.yaml", "server: http://from-file\n")
	t.Setenv("CONDUCTOR_SERVER_URL", "http://from-env")

	// "default" is an alias, so it must not flip the environment off the way a
	// named profile does.
	empty := NewResolution("", file, nil)
	alias := NewResolution("default", file, nil)
	if empty.SourceOf("server") != alias.SourceOf("server") {
		t.Errorf("--profile default disagrees with no profile: %+v vs %+v",
			alias.SourceOf("server"), empty.SourceOf("server"))
	}
}

func TestSourceOfFlagBeatsEverything(t *testing.T) {
	dir := t.TempDir()
	file := writeConfig(t, dir, "config.yaml", "server: http://from-file\n")
	t.Setenv("CONDUCTOR_SERVER_URL", "http://from-env")

	changed := func(key string) bool { return key == "server" }
	r := NewResolution("", file, changed)

	if got := r.SourceOf("server"); got.Kind != SourceFlag {
		t.Errorf("server source = %+v, want flag", got)
	}
}

func TestSourceOfEmptyFileValueIsNotASource(t *testing.T) {
	dir := t.TempDir()
	// A key present but blank supplies nothing; naming the file as the source
	// would send a user to edit a line that is not responsible.
	file := writeConfig(t, dir, "config.yaml", "server: \"\"\n")
	os.Unsetenv("CONDUCTOR_SERVER_URL")

	r := NewResolution("", file, nil)
	if got := r.SourceOf("server"); got.Kind != SourceDefault {
		t.Errorf("server source = %+v, want default", got)
	}
}

func TestSourceOfMissingFileFallsBackCleanly(t *testing.T) {
	os.Unsetenv("CONDUCTOR_SERVER_URL")
	r := NewResolution("", filepath.Join(t.TempDir(), "absent.yaml"), nil)
	if got := r.SourceOf("server"); got.Kind != SourceDefault {
		t.Errorf("server source = %+v, want default", got)
	}
}

func TestSourceString(t *testing.T) {
	tests := []struct {
		src  Source
		want string
	}{
		{Source{Kind: SourceFlag, Detail: "server"}, "flag --server"},
		{Source{Kind: SourceEnv, Detail: "CONDUCTOR_SERVER_URL"}, "env CONDUCTOR_SERVER_URL"},
		{Source{Kind: SourceFile, Detail: "/cfg/config.yaml"}, "/cfg/config.yaml"},
		{Source{Kind: SourceDefault}, "default"},
	}
	for _, tt := range tests {
		if got := tt.src.String(); got != tt.want {
			t.Errorf("Source%+v.String() = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestSourceShortString(t *testing.T) {
	// The table in `config show` prints the full path in its header, so repeating
	// it on every row only makes the column unreadable.
	s := Source{Kind: SourceFile, Detail: "/home/u/.conductor-cli/config.yaml"}
	if got := s.ShortString(); got != "config.yaml" {
		t.Errorf("ShortString() = %q, want config.yaml", got)
	}
	env := Source{Kind: SourceEnv, Detail: "CONDUCTOR_SERVER_URL"}
	if got := env.ShortString(); got != "env CONDUCTOR_SERVER_URL" {
		t.Errorf("ShortString() = %q, want the full env form", got)
	}
}

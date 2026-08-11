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

// Package cliconfig resolves which config file the CLI reads and writes, and
// where each resolved setting came from.
//
// Save, delete and initConfig each rebuilt the file name inline, which is how
// config.yaml ended up readable but unwritable (#98). They all go through
// Resolve now.
package cliconfig

import (
	"os"
	"path/filepath"
)

// DirName is the CLI's configuration directory, relative to the user's home.
const DirName = ".conductor-cli"

// DefaultProfileName is an alias for the default config file, not a profile of
// its own: `--profile default` and no --profile select the same file.
const DefaultProfileName = "default"

// defaultFileName is loaded when no named profile is selected. Stated
// explicitly: relying on viper's identical built-in default is what let the read
// path outlive the write path (#98).
const defaultFileName = "config"

// Keys are the settings the CLI resolves from flags, environment and file.
var Keys = []string{"server", "server-type", "auth-key", "auth-secret", "auth-token"}

// envVars mirrors the BindEnv calls in cmd/root.go. The two must agree, or
// reported sources lie.
var envVars = map[string]string{
	"server":      "CONDUCTOR_SERVER_URL",
	"server-type": "CONDUCTOR_SERVER_TYPE",
	"auth-key":    "CONDUCTOR_AUTH_KEY",
	"auth-secret": "CONDUCTOR_AUTH_SECRET",
	"auth-token":  "CONDUCTOR_AUTH_TOKEN",
}

// secretKeys are masked whenever a value is displayed.
var secretKeys = map[string]bool{
	"auth-secret": true,
	"auth-token":  true,
	"auth-key":    true,
}

// Dir returns the CLI configuration directory.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, DirName), nil
}

// IsDefault reports whether profile names the default config. "" and "default"
// both do.
func IsDefault(profile string) bool {
	return profile == "" || profile == DefaultProfileName
}

// FileName returns the bare file name for profile, as viper's SetConfigName
// expects it.
func FileName(profile string) string {
	if IsDefault(profile) {
		return defaultFileName
	}
	return "config-" + profile
}

// Resolve returns the path of profile's config file within dir: config.yaml for
// the default, config-<profile>.yaml otherwise.
func Resolve(dir, profile string) string {
	return filepath.Join(dir, FileName(profile)+".yaml")
}

// EnvVar returns the environment variable bound to key, or "" if key has none.
func EnvVar(key string) string {
	return envVars[key]
}

// EnvActive reports whether any config environment variable is set. The
// environment is all-or-nothing: one variable selects it for everything, and the
// default config file is not read.
//
// CONDUCTOR_PROFILE is excluded — it chooses a file rather than carrying a
// setting.
func EnvActive() bool {
	for _, env := range envVars {
		if os.Getenv(env) != "" {
			return true
		}
	}
	return false
}

// IsSecret reports whether key's value must be masked when displayed.
func IsSecret(key string) bool {
	return secretKeys[key]
}

// Mask hides a secret for display. Empty stays empty, so "unset" and "set but
// hidden" stay distinguishable.
func Mask(key, value string) string {
	if value != "" && IsSecret(key) {
		return "****"
	}
	return value
}

// StrayDefaultFile returns the path of a config-default.yaml in dir, or "".
// Earlier builds could create one; it is unreachable now that "default" aliases
// config.yaml, so callers warn rather than ignore it silently.
func StrayDefaultFile(dir string) string {
	path := filepath.Join(dir, "config-"+DefaultProfileName+".yaml")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

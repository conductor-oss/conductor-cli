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

// Package cliconfig resolves which configuration file the CLI reads and writes,
// and explains where each resolved setting came from.
//
// Both questions used to be answered in three different places — config save,
// config delete and initConfig each rebuilt the file name inline — which is how
// the default config.yaml ended up readable but unwritable (#98). Everything
// funnels through Resolve here so a rule can only be changed in one place.
package cliconfig

import (
	"os"
	"path/filepath"
)

// DirName is the CLI's configuration directory, relative to the user's home.
const DirName = ".conductor-cli"

// DefaultProfileName is the name that refers to the default configuration file.
// It is an alias, not a profile of its own: `--profile default` and no --profile
// at all select the same file and the same precedence rules.
const DefaultProfileName = "default"

// defaultFileName is the file the CLI loads when no named profile is selected.
// viper would find this name on its own, but relying on that default is what
// made the read path outlive the write path in #98, so it is stated explicitly.
const defaultFileName = "config"

// Keys are the settings the CLI resolves from flags, environment and file.
var Keys = []string{"server", "server-type", "auth-key", "auth-secret", "auth-token"}

// envVars maps a config key to the environment variable bound to it. It mirrors
// the BindEnv calls in cmd/root.go; the two must agree or reported sources lie.
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

// IsDefault reports whether profile names the default configuration. An empty
// name and "default" both do.
func IsDefault(profile string) bool {
	return profile == "" || profile == DefaultProfileName
}

// FileName returns the bare config file name for profile, without a directory
// or extension — the form viper's SetConfigName expects.
func FileName(profile string) string {
	if IsDefault(profile) {
		return defaultFileName
	}
	return "config-" + profile
}

// Resolve returns the absolute path of profile's config file within dir.
// The default configuration is config.yaml; every named profile is
// config-<profile>.yaml.
func Resolve(dir, profile string) string {
	return filepath.Join(dir, FileName(profile)+".yaml")
}

// EnvVar returns the environment variable bound to key, or "" if key has none.
func EnvVar(key string) string {
	return envVars[key]
}

// IsSecret reports whether key's value must be masked when displayed.
func IsSecret(key string) bool {
	return secretKeys[key]
}

// Mask renders value safely for display. Empty stays empty so that "unset" and
// "set but hidden" remain distinguishable.
func Mask(key, value string) string {
	if value != "" && IsSecret(key) {
		return "****"
	}
	return value
}

// StrayDefaultFile returns the path of a config-default.yaml in dir, or "" when
// there is none.
//
// "default" now aliases config.yaml, so a literal config-default.yaml is
// unreachable — earlier builds would happily create one via
// `config save --profile default`. It is reported so the CLI can warn instead
// of silently ignoring settings the user believes are active.
func StrayDefaultFile(dir string) string {
	path := filepath.Join(dir, "config-"+DefaultProfileName+".yaml")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

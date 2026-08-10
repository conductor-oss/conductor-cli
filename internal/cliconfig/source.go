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

	"gopkg.in/yaml.v3"
)

// SourceKind identifies where a resolved setting came from.
type SourceKind string

const (
	SourceFlag    SourceKind = "flag"
	SourceEnv     SourceKind = "env"
	SourceFile    SourceKind = "file"
	SourceDefault SourceKind = "default"
)

// Source is the origin of one resolved setting.
type Source struct {
	Kind SourceKind
	// Detail names the origin: the flag, the environment variable, or the
	// config file path. Empty for SourceDefault.
	Detail string
}

// String renders the source for display, e.g. "env CONDUCTOR_SERVER_URL". File
// sources carry their full path, for output that does not otherwise name it.
func (s Source) String() string {
	switch s.Kind {
	case SourceFlag:
		return "flag --" + s.Detail
	case SourceEnv:
		return "env " + s.Detail
	case SourceFile:
		return s.Detail
	default:
		return "default"
	}
}

// ShortString is String with file sources reduced to a bare file name, for
// output that already states the full path once.
func (s Source) ShortString() string {
	if s.Kind == SourceFile {
		return filepath.Base(s.Detail)
	}
	return s.String()
}

// Resolution is the configuration state the CLI resolved at startup: which file
// was loaded, whether environment variables participate, and which keys the file
// actually supplied. It answers "where did this value come from" per key.
type Resolution struct {
	// Profile is the effective profile name. Empty or "default" means the
	// default configuration.
	Profile string
	// File is the config file that was loaded, or "" when none was found.
	File string
	// EnvBound reports whether environment variables are in play. They are not
	// when a named profile is selected — cmd/root.go skips BindEnv so that an
	// explicitly chosen profile wins over ambient environment.
	EnvBound bool
	// fileKeys are the keys the loaded file supplied.
	fileKeys map[string]bool
	// flagChanged reports whether the user passed the flag for a key.
	flagChanged func(key string) bool
}

// NewResolution builds a Resolution for the given effective profile and config
// file. A file that cannot be read contributes no keys, matching the CLI's
// behaviour of running on flags and environment alone.
//
// flagChanged may be nil, in which case no key is attributed to a flag.
func NewResolution(profile, file string, flagChanged func(key string) bool) Resolution {
	if flagChanged == nil {
		flagChanged = func(string) bool { return false }
	}
	return Resolution{
		Profile:     profile,
		File:        file,
		EnvBound:    IsDefault(profile),
		fileKeys:    readKeys(file),
		flagChanged: flagChanged,
	}
}

// IsDefaultProfile reports whether the default configuration is in effect.
func (r Resolution) IsDefaultProfile() bool {
	return IsDefault(r.Profile)
}

// SourceOf returns where key's effective value came from.
//
// The order mirrors viper's precedence — flag, environment, file, default — with
// the environment step skipped when a named profile is active. Getting that skip
// wrong would report a set CONDUCTOR_SERVER_URL as the live source while the
// profile's value is the one actually used.
func (r Resolution) SourceOf(key string) Source {
	if r.flagChanged(key) {
		return Source{Kind: SourceFlag, Detail: key}
	}
	if r.EnvBound {
		if env := EnvVar(key); env != "" && os.Getenv(env) != "" {
			return Source{Kind: SourceEnv, Detail: env}
		}
	}
	if r.fileKeys[key] {
		return Source{Kind: SourceFile, Detail: r.File}
	}
	return Source{Kind: SourceDefault}
}

// readKeys returns the top-level keys present in the YAML file at path. A
// missing or malformed file yields no keys rather than an error: the CLI still
// runs on flags and environment, and the caller only needs to know what the file
// contributed.
func readKeys(path string) map[string]bool {
	keys := map[string]bool{}
	if path == "" {
		return keys
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return keys
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return keys
	}
	for k, v := range raw {
		// A key present but empty supplies nothing, and reporting it as the
		// source would point at a file that did not set the value.
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		keys[k] = true
	}
	return keys
}

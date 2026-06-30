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

// Package deploy bridges Conductor's connection config into the environment of the
// language-specific (Python/TypeScript) deploy subprocess, and abstracts subprocess
// execution behind an interface. The env-var names are named constants — never inline
// literals — and the server URL/token come from config, not hardcoded values.
package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Subprocess environment variables the AgentSpan SDKs read.
const (
	EnvServerURL = "AGENTSPAN_SERVER_URL"
	EnvAuthToken = "AGENTSPAN_AUTH_TOKEN"
	EnvAutoStart = "AGENTSPAN_AUTO_START_SERVER"

	envPrefix    = "AGENTSPAN_"
	autoStartOff = "false"

	subprocessTimeout = 120 * time.Second
)

// TokenSource yields the JWT to forward to the subprocess. It matches
// transport.TokenProvider structurally, so the shared transport's provider can be
// passed directly.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// EnvBuilder builds the subprocess environment from Conductor's connection config:
// it strips inherited AGENTSPAN_* vars, sets the server URL, disables the SDK's
// embedded-server auto-start, and forwards the resolved JWT as AGENTSPAN_AUTH_TOKEN.
//
// Cross-component note (design §7): forwarding the JWT — rather than a legacy API
// key — requires the Python/TypeScript SDKs to authenticate with AGENTSPAN_AUTH_TOKEN.
// This is the single cross-component dependency of the port.
type EnvBuilder struct {
	BaseURL string
	Tokens  TokenSource // nil => anonymous
	BaseEnv []string    // typically os.Environ(); injected for testability
}

// Build returns the child-process environment.
func (b EnvBuilder) Build(ctx context.Context) ([]string, error) {
	out := make([]string, 0, len(b.BaseEnv)+3)
	for _, e := range b.BaseEnv {
		if !strings.HasPrefix(e, envPrefix) {
			out = append(out, e)
		}
	}
	out = append(out, EnvServerURL+"="+b.BaseURL, EnvAutoStart+"="+autoStartOff)
	if b.Tokens != nil {
		tok, err := b.Tokens.Token(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve auth token: %w", err)
		}
		if tok != "" {
			out = append(out, EnvAuthToken+"="+tok)
		}
	}
	return out, nil
}

// Runner executes a language subprocess and captures stdout, forwarding stderr to the
// user. It is an interface so the deploy command can be tested without spawning processes.
type Runner interface {
	Run(ctx context.Context, env []string, name string, args ...string) ([]byte, error)
}

// NewRunner returns the default subprocess Runner.
func NewRunner() Runner { return execRunner{} }

type execRunner struct{}

func (execRunner) Run(ctx context.Context, env []string, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, subprocessTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil && stdout.Len() > 0 {
		// Subprocess may have written results before failing — return partial output.
		return stdout.Bytes(), err
	}
	if err != nil {
		return nil, fmt.Errorf("run %s: %w", name, err)
	}
	return stdout.Bytes(), nil
}

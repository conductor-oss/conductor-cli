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

// Package providers reads the server's view of which AI providers it can dial.
//
// Agents execute on the server, which resolves provider credentials from its own
// environment, credential store, or properties. A client shell's OPENAI_API_KEY says
// nothing about that, so the CLI asks rather than guesses.
//
// The transport applies no timeout of its own (request lifetime belongs to the
// caller's context), so callers bound this themselves — a diagnostic command must not
// hang on an unreachable server.
package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

const statusPath = "/providers/status"

// ErrUnsupported means the server has no provider-status route: AI integrations are
// disabled, or the deployment predates the endpoint. It describes the deployment, not
// a failure, and callers are expected to report it and carry on.
var ErrUnsupported = errors.New("server does not expose provider status")

// Status is the server's provider configuration.
type Status struct {
	// ManagedByHost is true when the server is embedded in a host that owns provider
	// configuration. Providers is then empty by design rather than by omission; the
	// wire contract is forward-compatible, so per-provider detail may appear later.
	ManagedByHost bool       `json:"managedByHost"`
	Providers     []Provider `json:"providers"`
}

// Provider is one provider's status as the server sees it.
type Provider struct {
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	// BaseURL is reported only for URL-based providers (ollama).
	BaseURL string `json:"baseUrl,omitempty"`
	// Reachable is probed from the server's own network — a fact no client can
	// observe for itself. It is a pointer because "not probed" and "probed, and it
	// failed" are different answers.
	Reachable *bool `json:"reachable,omitempty"`
}

// Fetch reads provider status from the server. A missing route yields ErrUnsupported.
func Fetch(ctx context.Context, t transport.Config) (Status, error) {
	var st Status
	if err := t.DoJSON(ctx, http.MethodGet, statusPath, nil, &st); err != nil {
		var apiErr *transport.APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
			return Status{}, ErrUnsupported
		}
		return Status{}, fmt.Errorf("read provider status: %w", err)
	}
	return st, nil
}

// Configured returns the names of the providers the server reports as configured.
func (s Status) Configured() []string {
	var names []string
	for _, p := range s.Providers {
		if p.Configured {
			names = append(names, p.Name)
		}
	}
	return names
}

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
	"context"
	"net/http"

	"github.com/conductor-sdk/conductor-go/sdk/settings"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

// sdkTokenManager is the subset of the conductor-go TokenManager interface that the
// CLI's token managers implement (ConfigTokenManager and *CachedTokenManager). It
// mints or refreshes the JWT used for X-Authorization.
type sdkTokenManager interface {
	RefreshToken(*settings.HttpSettings, *http.Client) (string, error)
}

// tokenProvider adapts a conductor-go token manager to transport.TokenProvider so
// that agent and skill traffic reuses exactly the same JWT — including refresh and
// caching — as the conductor-go SDK client.
type tokenProvider struct {
	manager      sdkTokenManager
	httpSettings *settings.HttpSettings
	httpClient   *http.Client
}

// newTokenProvider wraps a token manager. Pass nil to mean anonymous access.
func newTokenProvider(m sdkTokenManager, hs *settings.HttpSettings) transport.TokenProvider {
	if m == nil {
		return nil
	}
	return tokenProvider{manager: m, httpSettings: hs, httpClient: &http.Client{}}
}

func (p tokenProvider) Token(context.Context) (string, error) {
	return p.manager.RefreshToken(p.httpSettings, p.httpClient)
}

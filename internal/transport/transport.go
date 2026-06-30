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

// Package transport is the shared HTTP transport for the agent and skill clients.
// Those endpoints are not part of the conductor-go SDK, so the CLI owns a small,
// interface-bounded transport that reuses Conductor's resolved server URL and JWT
// (see cmd/root.go) — one backend, one auth path. No file paths, ports, or URLs are
// hardcoded here: BaseURL comes from config and request lifetime from the context.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// authHeader is the header Conductor uses to carry the bearer JWT. It matches the
// conductor-go SDK (sdk/client/http_requester.go) and the merged server's auth filter.
const authHeader = "X-Authorization"

// TokenProvider yields the current JWT for the X-Authorization header. An empty
// string with a nil error means anonymous access — the header is omitted.
type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// Config is the shared transport for agent and skill traffic. BaseURL and Tokens are
// resolved once at startup from the same source as the conductor-go client, so these
// calls reuse Conductor's server URL and authentication — no second config, no second
// auth path.
type Config struct {
	BaseURL string
	Tokens  TokenProvider // nil => anonymous
	HTTP    *http.Client  // nil => defaultClient
}

// defaultClient has no timeout on purpose: request lifetime is governed by the
// caller's context, which also keeps long-lived SSE streams open until cancelled.
var defaultClient = &http.Client{}

func (c Config) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return defaultClient
}

// Do builds and executes a request against BaseURL+path. It attaches the
// X-Authorization header (when a token is available) plus any extra headers, then
// returns the response unchanged for a 2xx status, or a normalized *APIError for a
// non-2xx status (whose body it consumes). On success the caller owns closing
// resp.Body, which makes Do equally suitable for streaming (SSE) responses.
func (c Config) Do(ctx context.Context, method, path string, body io.Reader, header http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	if err := c.applyAuth(ctx, req); err != nil {
		return nil, err
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, newAPIError(method, path, resp) // consumes & closes the body
	}
	return resp, nil
}

// DoJSON marshals reqBody (when non-nil) as JSON, executes the request, and decodes a
// 2xx response into respBody (when non-nil). Pass a *json.RawMessage as respBody to
// capture the raw payload without imposing a schema across the layer boundary.
func (c Config) DoJSON(ctx context.Context, method, path string, reqBody, respBody any) error {
	var body io.Reader
	header := http.Header{}
	if reqBody != nil {
		data, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(data)
		header.Set("Content-Type", "application/json")
	}

	resp, err := c.Do(ctx, method, path, body, header)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if respBody == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}

func (c Config) applyAuth(ctx context.Context, req *http.Request) error {
	if c.Tokens == nil {
		return nil
	}
	tok, err := c.Tokens.Token(ctx)
	if err != nil {
		return fmt.Errorf("resolve auth token: %w", err)
	}
	if tok != "" {
		req.Header.Set(authHeader, tok)
	}
	return nil
}

// APIError is a non-2xx response from the Conductor server. It mirrors the server's
// JSON error shape ({status, message, error}) so agent and skill errors read
// consistently with the rest of the CLI (cf. cmd.parseAPIError, which does the same
// for SDK-surfaced errors).
type APIError struct {
	Status  int
	Method  string
	Path    string
	Message string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s (status %d)", e.Message, e.Status)
	}
	return fmt.Sprintf("%s %s failed with status %d", e.Method, e.Path, e.Status)
}

// newAPIError reads and closes resp.Body and builds an *APIError from it.
func newAPIError(method, path string, resp *http.Response) *APIError {
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	apiErr := &APIError{Status: resp.StatusCode, Method: method, Path: path}

	var parsed struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if json.Unmarshal(raw, &parsed) == nil {
		switch {
		case parsed.Message != "":
			apiErr.Message = parsed.Message
		case parsed.Error != "":
			apiErr.Message = parsed.Error
		}
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(raw))
	}
	return apiErr
}

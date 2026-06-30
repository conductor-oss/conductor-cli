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

package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

// Endpoint paths and fixed tokens — named constants, never inline literals.
// pathSkills is relative to the transport BaseURL, which already carries the
// "/api" prefix (see cmd/root.go); do not re-add it here.
const (
	pathSkills       = "/skills"
	registerSegment  = "/register"
	versionSegment   = "/versions/"
	packageSegment   = "/package"
	queryAllVersions = "allVersions"
	valueTrue        = "true"
	versionLatest    = "latest"

	fieldManifest    = "manifest"
	fieldPackage     = "package"
	packageFileName  = "skill.zip"
	headerContentTyp = "Content-Type"
)

// Client is the transport boundary for skill endpoints. It returns domain types or
// raw package bytes — never *http.Response or transport types.
type Client interface {
	List(ctx context.Context, allVersions bool) ([]Summary, error)
	Get(ctx context.Context, name, version string) (Detail, error)
	DownloadPackage(ctx context.Context, name, version string) ([]byte, error)
	Register(ctx context.Context, manifest json.RawMessage, pkg []byte) (Detail, error)
	Delete(ctx context.Context, name, version string) error
}

// NewClient returns a Client backed by the shared transport.
func NewClient(t transport.Config) Client {
	return &restClient{t: t}
}

type restClient struct {
	t transport.Config
}

func (c *restClient) List(ctx context.Context, allVersions bool) ([]Summary, error) {
	path := pathSkills
	if allVersions {
		q := url.Values{}
		q.Set(queryAllVersions, valueTrue)
		path += "?" + q.Encode()
	}
	var out []Summary
	if err := c.t.DoJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *restClient) Get(ctx context.Context, name, version string) (Detail, error) {
	path := pathSkills + "/" + url.PathEscape(name)
	if version != "" {
		path += versionSegment + url.PathEscape(version)
	}
	var out Detail
	if err := c.t.DoJSON(ctx, http.MethodGet, path, nil, &out); err != nil {
		return Detail{}, err
	}
	return out, nil
}

func (c *restClient) DownloadPackage(ctx context.Context, name, version string) ([]byte, error) {
	path := pathSkills + "/" + url.PathEscape(name) + versionSegment + url.PathEscape(resolveVersion(version)) + packageSegment
	resp, err := c.t.Do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// Register uploads a skill manifest (JSON) and its package (zip) as multipart form
// data. The manifest is built by the cmd layer from the local skill directory; the
// client only frames the request — it never touches the filesystem.
func (c *restClient) Register(ctx context.Context, manifest json.RawMessage, pkg []byte) (Detail, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	mf, err := w.CreateFormField(fieldManifest)
	if err != nil {
		return Detail{}, err
	}
	if _, err := mf.Write(manifest); err != nil {
		return Detail{}, err
	}
	pf, err := w.CreateFormFile(fieldPackage, packageFileName)
	if err != nil {
		return Detail{}, err
	}
	if _, err := pf.Write(pkg); err != nil {
		return Detail{}, err
	}
	if err := w.Close(); err != nil {
		return Detail{}, fmt.Errorf("close multipart body: %w", err)
	}

	header := http.Header{}
	header.Set(headerContentTyp, w.FormDataContentType())
	resp, err := c.t.Do(ctx, http.MethodPost, pathSkills+registerSegment, &body, header)
	if err != nil {
		return Detail{}, err
	}
	defer resp.Body.Close()
	var out Detail
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Detail{}, fmt.Errorf("decode response: %w", err)
	}
	return out, nil
}

func (c *restClient) Delete(ctx context.Context, name, version string) error {
	path := pathSkills + "/" + url.PathEscape(name) + versionSegment + url.PathEscape(resolveVersion(version))
	return c.t.DoJSON(ctx, http.MethodDelete, path, nil, nil)
}

// resolveVersion defaults an empty version to "latest", matching the server's
// version-pinned package and delete endpoints.
func resolveVersion(version string) string {
	if version == "" {
		return versionLatest
	}
	return version
}

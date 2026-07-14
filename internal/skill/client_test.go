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
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/conductor-oss/conductor-cli/internal/transport"
)

func newTestClient(t *testing.T, h http.HandlerFunc) Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(transport.Config{BaseURL: srv.URL})
}

func TestListSkills(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathSkills {
			t.Errorf("path = %q, want %q", r.URL.Path, pathSkills)
		}
		if r.URL.Query().Get(queryAllVersions) != valueTrue {
			t.Errorf("expected allVersions=true, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[{"name":"summarize","version":"abc123","fileCount":3}]`))
	})
	out, err := c.List(context.Background(), true)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(out) != 1 || out[0].Name != "summarize" || out[0].FileCount != 3 {
		t.Errorf("got %+v", out)
	}
}

func TestGetSkillWithVersionPath(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := pathSkills + "/summarize" + versionSegment + "v1"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		_, _ = w.Write([]byte(`{"name":"summarize","version":"v1"}`))
	})
	d, err := c.Get(context.Background(), "summarize", "v1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if d.Version != "v1" {
		t.Errorf("version = %q, want v1", d.Version)
	}
}

func TestDownloadPackageDefaultsToLatest(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		want := pathSkills + "/summarize" + versionSegment + versionLatest + packageSegment
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		_, _ = w.Write([]byte("ZIPBYTES"))
	})
	data, err := c.DownloadPackage(context.Background(), "summarize", "")
	if err != nil {
		t.Fatalf("DownloadPackage: %v", err)
	}
	if string(data) != "ZIPBYTES" {
		t.Errorf("got %q", data)
	}
}

func TestRegisterSendsMultipart(t *testing.T) {
	var gotManifest string
	var gotPackage []byte
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathSkills+registerSegment {
			t.Errorf("path = %q, want %q", r.URL.Path, pathSkills+registerSegment)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse multipart: %v", err)
		}
		gotManifest = r.FormValue(fieldManifest)
		file, _, err := r.FormFile(fieldPackage)
		if err != nil {
			t.Fatalf("form file: %v", err)
		}
		defer file.Close()
		gotPackage, _ = io.ReadAll(file)
		_, _ = w.Write([]byte(`{"name":"summarize","version":"v1"}`))
	})

	detail, err := c.Register(context.Background(), []byte(`{"name":"summarize"}`), []byte("ZIPDATA"))
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if detail.Version != "v1" {
		t.Errorf("version = %q, want v1", detail.Version)
	}
	if gotManifest != `{"name":"summarize"}` {
		t.Errorf("manifest = %q", gotManifest)
	}
	if string(gotPackage) != "ZIPDATA" {
		t.Errorf("package = %q", gotPackage)
	}
}

func TestDeleteSkill(t *testing.T) {
	called := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		want := pathSkills + "/summarize" + versionSegment + "v2"
		if r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
	})
	if err := c.Delete(context.Background(), "summarize", "v2"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !called {
		t.Error("delete request was not made")
	}
}

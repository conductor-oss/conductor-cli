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
	"encoding/json"
)

// Service is the skill use-case layer. It depends only on Client and is free of
// presentation and transport concerns.
type Service interface {
	List(ctx context.Context, allVersions bool) ([]Summary, error)
	Get(ctx context.Context, name, version string) (Detail, error)
	DownloadPackage(ctx context.Context, name, version string) ([]byte, error)
	Register(ctx context.Context, manifest json.RawMessage, pkg []byte) (Detail, error)
	Delete(ctx context.Context, name, version string) error
}

// NewService returns a Service backed by the given Client.
func NewService(c Client) Service {
	return &service{client: c}
}

type service struct {
	client Client
}

func (s *service) List(ctx context.Context, allVersions bool) ([]Summary, error) {
	return s.client.List(ctx, allVersions)
}

func (s *service) Get(ctx context.Context, name, version string) (Detail, error) {
	return s.client.Get(ctx, name, version)
}

func (s *service) DownloadPackage(ctx context.Context, name, version string) ([]byte, error) {
	return s.client.DownloadPackage(ctx, name, version)
}

func (s *service) Register(ctx context.Context, manifest json.RawMessage, pkg []byte) (Detail, error) {
	return s.client.Register(ctx, manifest, pkg)
}

func (s *service) Delete(ctx context.Context, name, version string) error {
	return s.client.Delete(ctx, name, version)
}

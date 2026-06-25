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
	"strings"
	"testing"
)

func TestSignalUnsupportedOnOSS(t *testing.T) {
	// Case-insensitivity of serverType is the contract of isEnterpriseServer
	// and is covered by TestIsEnterpriseServer. Here we only assert the two
	// behaviors this function owns: error on non-Enterprise, nil on Enterprise.
	tests := []struct {
		name      string
		serverVal string
		wantErr   bool
	}{
		{name: "OSS returns error", serverVal: "OSS", wantErr: true},
		{name: "Enterprise returns nil", serverVal: "Enterprise", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldServerType := serverType
			defer func() { serverType = oldServerType }()
			serverType = tt.serverVal

			err := signalUnsupportedOnOSS()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("signalUnsupportedOnOSS() with serverType=%q = nil, want error", tt.serverVal)
				}
				if !strings.Contains(err.Error(), "not supported in OSS Conductor") {
					t.Errorf("error %q does not mention OSS Conductor", err.Error())
				}
			} else if err != nil {
				t.Errorf("signalUnsupportedOnOSS() with serverType=%q = %v, want nil", tt.serverVal, err)
			}
		})
	}
}

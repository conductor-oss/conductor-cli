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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/conductor-sdk/conductor-go/sdk/authentication"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:     "whoami",
	Short:   "Display information about the current user",
	GroupID: "config",
	RunE: func(cmd *cobra.Command, args []string) error {
		var jwtToken string

		// Get token from auth-token or key/secret
		if token != "" {
			jwtToken = token
		} else if key != "" && secret != "" {
			// Get token using SDK's authentication.GetToken
			tokenResponse, _, err := authentication.GetToken(
				*settings.NewAuthenticationSettings(key, secret),
				settings.NewHttpSettings(url),
				http.DefaultClient,
			)
			if err != nil {
				return fmt.Errorf("failed to get token from API: %v", err)
			}
			jwtToken = tokenResponse.Token
		} else {
			return fmt.Errorf("no authentication configured - please configure auth-token or auth-key/auth-secret")
		}

		// Print server URL
		fmt.Println(url)

		// Decode JWT token (without verification, just reading claims)
		claims, err := decodeJWT(jwtToken)
		if err != nil {
			return fmt.Errorf("failed to decode JWT token: %v", err)
		}

		// Try to extract email and name
		email, emailOk := claims["email"].(string)
		name, nameOk := claims["name"].(string)

		if nameOk && name != "" {
			fmt.Println(name)
		}

		if emailOk && email != "" {
			fmt.Println(email)
		}

		// If email and name are not available, check for sub
		if (!emailOk || email == "") && (!nameOk || name == "") {
			if sub, ok := claims["sub"].(string); ok && sub != "" {
				fmt.Println(sub)
			}
		}

		// If nothing was printed, show error
		if (!emailOk || email == "") && (!nameOk || name == "") {
			if sub, ok := claims["sub"].(string); !ok || sub == "" {
				return fmt.Errorf("no user information found in token (email, name, or sub)")
			}
		}

		return nil
	},
	SilenceUsage: true,
}

func decodeJWT(tokenString string) (map[string]interface{}, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payload := parts[1]

	// Add padding if needed (base64 URL encoding may omit padding)
	if l := len(payload) % 4; l > 0 {
		payload += strings.Repeat("=", 4-l)
	}

	// Decode from base64 URL encoding
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try standard base64 if URL encoding fails
		decoded, err = base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64: %v", err)
		}
	}

	// Parse JSON
	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return claims, nil
}

func init() {
	rootCmd.AddCommand(whoamiCmd)
}

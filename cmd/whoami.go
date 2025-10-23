package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display information about the current user",
	RunE: func(cmd *cobra.Command, args []string) error {
		var jwtToken string

		// Get token from auth-token or key/secret
		if token != "" {
			jwtToken = token
		} else if key != "" && secret != "" {
			// TODO: Get token from SDK's token manager
			// Need to access: apiClient.TokenManager or similar
			return fmt.Errorf("whoami with API key/secret not yet implemented")
		} else {
			return fmt.Errorf("no authentication configured - please configure auth-token or auth-key/auth-secret")
		}

		// Decode JWT token (without verification, just reading claims)
		claims, err := decodeJWT(jwtToken)
		if err != nil {
			return fmt.Errorf("failed to decode JWT token: %v", err)
		}

		// Try to extract email and name
		email, emailOk := claims["email"].(string)
		name, nameOk := claims["name"].(string)

		if emailOk && email != "" {
			fmt.Printf("Email: %s\n", email)
		}
		if nameOk && name != "" {
			fmt.Printf("Name: %s\n", name)
		}

		// If email and name are not available, check for sub
		if (!emailOk || email == "") && (!nameOk || name == "") {
			if sub, ok := claims["sub"].(string); ok && sub != "" {
				fmt.Printf("Subject: %s\n", sub)
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

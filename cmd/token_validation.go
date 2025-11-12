package cmd

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
)

// getTokenExpiry extracts the expiry time from a JWT token
// Returns the expiry as Unix timestamp in seconds, or an error if parsing fails
func getTokenExpiry(tokenString string) (int64, error) {
	// Parse JWT without validation (we only need to read claims)
	token, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		// If JWT parsing fails, try manual base64 decode as fallback
		return getTokenExpiryManual(tokenString)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, fmt.Errorf("invalid token claims")
	}

	exp, ok := claims["exp"]
	if !ok {
		return 0, fmt.Errorf("token does not contain exp claim")
	}

	switch v := exp.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	case json.Number:
		expInt, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("invalid exp claim format: %v", err)
		}
		return expInt, nil
	default:
		return 0, fmt.Errorf("exp claim has unexpected type: %T", exp)
	}
}

// getTokenExpiryManual manually decodes JWT payload to extract expiry
// Used as fallback when standard JWT parsing fails
func getTokenExpiryManual(tokenString string) (int64, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return 0, fmt.Errorf("invalid JWT format: expected 3 parts, got %d", len(parts))
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("failed to decode JWT payload: %v", err)
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return 0, fmt.Errorf("failed to parse JWT claims: %v", err)
	}

	exp, ok := claims["exp"]
	if !ok {
		return 0, fmt.Errorf("token does not contain exp claim")
	}

	switch v := exp.(type) {
	case float64:
		return int64(v), nil
	case int64:
		return v, nil
	default:
		return 0, fmt.Errorf("exp claim has unexpected type: %T", exp)
	}
}

// isTokenExpired checks if a token is expired or will expire soon
// bufferSeconds is the number of seconds before expiry to consider token expired (for proactive refresh)
// Returns false for long-lived tokens (expiryUnix == -1)
func isTokenExpired(expiryUnix int64, bufferSeconds int64) bool {
	// Long-lived tokens (no expiry claim) never expire
	if expiryUnix == -1 {
		return false
	}

	// Unknown expiry (0) is considered expired
	if expiryUnix == 0 {
		return true
	}

	now := time.Now().Unix()
	return now >= (expiryUnix - bufferSeconds)
}

// validateUserToken validates a user-configured auth-token
// Returns error if token is expired
func validateUserToken(tokenString string) error {
	if tokenString == "" {
		return fmt.Errorf("token is empty")
	}

	expiry, err := getTokenExpiry(tokenString)
	if err != nil {
		// If we can't parse the token, we can't validate it
		// This might not be a JWT, so we allow it to pass
		// The server will reject it if it's invalid
		log.Debugf("Could not parse token expiry: %v", err)
		return nil
	}

	if isTokenExpired(expiry, 0) {
		return fmt.Errorf("your token has expired\nPlease check your authentication settings. Run 'orkes config save' to configure credentials")
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d minutes", seconds/60)
	}
	if seconds < 86400 {
		return fmt.Sprintf("%.1f hours", float64(seconds)/3600)
	}
	return fmt.Sprintf("%.1f days", float64(seconds)/86400)
}

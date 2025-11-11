package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/conductor-sdk/conductor-go/sdk/authentication"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	// Token expiry buffer: refresh token 5 minutes before it expires
	tokenExpiryBufferSeconds = 300 // 5 minutes
)

// CachedTokenManager manages authentication tokens with caching
// It implements the TokenManager interface from conductor-go SDK
type CachedTokenManager struct {
	Key          string
	Secret       string
	cachedToken  string
	tokenExpiry  int64
	configPath   string
	httpSettings *settings.HttpSettings
	mu           sync.RWMutex // Protects cachedToken and tokenExpiry
}

// NewCachedTokenManager creates a new CachedTokenManager
func NewCachedTokenManager(key, secret, cachedToken string, tokenExpiry int64, configPath string, httpSettings *settings.HttpSettings) *CachedTokenManager {
	return &CachedTokenManager{
		Key:          key,
		Secret:       secret,
		cachedToken:  cachedToken,
		tokenExpiry:  tokenExpiry,
		configPath:   configPath,
		httpSettings: httpSettings,
	}
}

// RefreshToken implements the TokenManager interface
// Returns a valid token, fetching a new one if the cached token is expired
func (m *CachedTokenManager) RefreshToken(httpSettings *settings.HttpSettings, httpClient *http.Client) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we have a valid cached token
	if m.cachedToken != "" && !isTokenExpired(m.tokenExpiry, tokenExpiryBufferSeconds) {
		if verbose {
			timeUntilExpiry := m.tokenExpiry - getCurrentTimeUnix()
			log.Debugf("Using cached token (expires in %s)", formatDuration(timeUntilExpiry))
		}
		return m.cachedToken, nil
	}

	// Cached token is missing or expired, fetch a new one
	if verbose {
		if m.cachedToken == "" {
			log.Debug("No cached token found, fetching new token...")
		} else {
			log.Debug("Cached token expired, fetching new token...")
		}
	}

	token, err := m.fetchNewToken(httpSettings, httpClient)
	if err != nil {
		return "", fmt.Errorf("failed to fetch new token: %w", err)
	}

	// Parse expiry from new token
	expiry, err := getTokenExpiry(token)
	if err != nil {
		log.Debugf("Warning: could not parse token expiry: %v", err)
		// Continue anyway - we have a token, just can't cache it optimally
		expiry = 0
	}

	// Update cache
	m.cachedToken = token
	m.tokenExpiry = expiry

	// Save to config file
	if err := m.saveCachedToken(); err != nil {
		// Log warning but don't fail - caching is optional
		log.Debugf("Warning: failed to save cached token: %v", err)
	}

	if verbose && expiry > 0 {
		timeUntilExpiry := expiry - getCurrentTimeUnix()
		log.Debugf("Fetched new token (expires in %s)", formatDuration(timeUntilExpiry))
	}

	return token, nil
}

// fetchNewToken requests a new token from the authentication service
func (m *CachedTokenManager) fetchNewToken(httpSettings *settings.HttpSettings, httpClient *http.Client) (string, error) {
	if m.Key == "" || m.Secret == "" {
		return "", fmt.Errorf("missing authentication credentials")
	}

	tokenResponse, _, err := authentication.GetToken(
		*settings.NewAuthenticationSettings(m.Key, m.Secret),
		httpSettings,
		httpClient,
	)
	if err != nil {
		return "", err
	}

	return tokenResponse.Token, nil
}

// saveCachedToken persists the cached token and expiry to the config file
func (m *CachedTokenManager) saveCachedToken() error {
	if m.configPath == "" {
		return fmt.Errorf("config path not set")
	}

	// Read existing config
	configData, err := m.readConfigFile()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Update only cache fields
	configData["cached-token"] = m.cachedToken
	configData["cached-token-expiry"] = m.tokenExpiry

	// Write back to file
	data, err := yaml.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// readConfigFile reads the current config file
func (m *CachedTokenManager) readConfigFile() (map[string]interface{}, error) {
	// Check if file exists
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// File doesn't exist, return empty config
		return make(map[string]interface{}), nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// getCurrentTimeUnix returns current Unix timestamp in seconds
func getCurrentTimeUnix() int64 {
	return time.Now().Unix()
}

// getConfigPath returns the path to the config file for the given profile
func getConfigPath(profileName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	configDir := filepath.Join(home, ".conductor-cli")
	configFileName := "config.yaml"
	if profileName != "" {
		configFileName = fmt.Sprintf("config-%s.yaml", profileName)
	}

	return filepath.Join(configDir, configFileName), nil
}

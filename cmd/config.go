package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "CLI configuration management",
	GroupID: "config",
}

var configSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save configuration to file (interactive)",
	Long: `Interactively configure and save server and authentication settings to a configuration file.

The configuration will be saved to ~/.conductor-cli/config.yaml by default,
or to a profile-specific file if --profile is specified.

If a configuration already exists, you can press Enter to keep existing values.

Examples:
  # Interactively save to default config file
  orkes config save

  # Interactively save to a named profile
  orkes config save --profile production
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := profile

		if err := interactiveSaveConfig(profileName); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		configFileName := "config.yaml"
		if profileName != "" {
			configFileName = fmt.Sprintf("config-%s.yaml", profileName)
		}

		fmt.Fprintf(os.Stdout, "✓ Configuration saved to ~/.conductor-cli/%s\n", configFileName)
		return nil
	},
	SilenceUsage: true,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration profiles",
	Long: `List all configuration profiles in ~/.conductor-cli directory.

Shows the default config.yaml and all named profiles (config-<profile>.yaml).

Examples:
  # List all config profiles
  orkes config list
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		configDir := filepath.Join(home, ".conductor-cli")

		// Check if config directory exists
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			fmt.Println("No configuration files found")
			return nil
		}

		// Read all files in config directory
		files, err := os.ReadDir(configDir)
		if err != nil {
			return fmt.Errorf("failed to read config directory: %w", err)
		}

		hasConfigs := false
		for _, file := range files {
			if file.IsDir() {
				continue
			}

			name := file.Name()

			// Handle default config.yaml
			if name == "config.yaml" {
				fmt.Println("default")
				hasConfigs = true
				continue
			}

			// Handle named profiles: config-<profile>.yaml
			if strings.HasPrefix(name, "config-") && strings.HasSuffix(name, ".yaml") {
				profileName := strings.TrimPrefix(name, "config-")
				profileName = strings.TrimSuffix(profileName, ".yaml")
				fmt.Println(profileName)
				hasConfigs = true
			}
		}

		if !hasConfigs {
			fmt.Println("No configuration files found")
		}

		return nil
	},
	SilenceUsage: true,
}

var configDeleteCmd = &cobra.Command{
	Use:   "delete [profile]",
	Short: "Delete a configuration file",
	Long: `Delete a configuration file.

If no profile is specified, the default config.yaml will be deleted.
Profile can be specified either as a positional argument or via --profile flag.

Examples:
  # Delete default config file (requires confirmation)
  orkes config delete

  # Delete a named profile using positional argument
  orkes config delete production

  # Delete a named profile using --profile flag
  orkes config delete --profile production

  # Delete without confirmation
  orkes config delete production -y
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		configDir := filepath.Join(home, ".conductor-cli")

		// Get profile name from either positional arg or --profile flag
		var profileName string
		if len(args) > 0 {
			profileName = args[0]
		} else if profile != "" {
			profileName = profile
		}

		configFileName := "config.yaml"
		if profileName != "" {
			configFileName = fmt.Sprintf("config-%s.yaml", profileName)
		}

		configPath := filepath.Join(configDir, configFileName)

		// Check if config file exists
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config file does not exist: %s", configPath)
		}

		// Ask for confirmation unless -y flag is set
		if !yes {
			reader := bufio.NewReader(os.Stdin)
			fmt.Fprintf(os.Stdout, "Are you sure you want to delete %s? [y/N]: ", configPath)
			response, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Fprintf(os.Stdout, "Deletion cancelled\n")
				return nil
			}
		}

		// Delete the file
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("failed to delete config file: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ Configuration deleted: %s\n", configPath)
		return nil
	},
	SilenceUsage: true,
}

func interactiveSaveConfig(profileName string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".conductor-cli")

	// Create config directory if it doesn't exist (0700 for security - contains credentials)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	// Determine config file name
	configFileName := "config.yaml"
	if profileName != "" {
		configFileName = fmt.Sprintf("config-%s.yaml", profileName)
	}

	configPath := filepath.Join(configDir, configFileName)

	// Load existing config if it exists
	existingConfig := make(map[string]string)
	if data, err := os.ReadFile(configPath); err == nil {
		var rawConfig map[string]interface{}
		if err := yaml.Unmarshal(data, &rawConfig); err == nil {
			for k, v := range rawConfig {
				if str, ok := v.(string); ok {
					existingConfig[k] = str
				}
			}
		}
	}

	reader := bufio.NewReader(os.Stdin)

	// Prompt for server URL
	serverDefault := existingConfig["server"]
	if serverDefault == "" {
		serverDefault = "http://localhost:8080/api"
	}
	fmt.Fprintf(os.Stdout, "Server URL [%s]: ", serverDefault)
	serverInput, _ := reader.ReadString('\n')
	serverInput = strings.TrimSpace(serverInput)
	server := serverDefault
	if serverInput != "" {
		server = serverInput
	}

	// Prompt for server type
	serverTypeDefault := existingConfig["server-type"]
	if serverTypeDefault == "" {
		serverTypeDefault = "Enterprise"
	}
	fmt.Fprintf(os.Stdout, "Server type (OSS/Enterprise) [%s]: ", serverTypeDefault)
	serverTypeInput, _ := reader.ReadString('\n')
	serverTypeInput = strings.TrimSpace(serverTypeInput)
	serverType := serverTypeDefault
	if serverTypeInput != "" {
		serverType = serverTypeInput
	}

	templateURLDefault := existingConfig["template-url"]
	if templateURLDefault == "" {
		templateURLDefault = "https://d2ozrtblsovn5m.cloudfront.net"
	}
	fmt.Fprintf(os.Stdout, "Template repo URL [%s]: ", templateURLDefault)
	templateURLInput, _ := reader.ReadString('\n')
	templateURLInput = strings.TrimSpace(templateURLInput)
	templateURL := templateURLDefault
	if templateURLInput != "" {
		templateURL = templateURLInput
	}

	// Prompt for auth method
	fmt.Fprintf(os.Stdout, "\nAuthentication method:\n")
	fmt.Fprintf(os.Stdout, "  1. API Key + Secret\n")
	fmt.Fprintf(os.Stdout, "  2. Auth Token\n")

	// Determine default auth method based on existing config
	defaultAuthMethod := "1"
	if existingConfig["auth-token"] != "" {
		defaultAuthMethod = "2"
	}

	fmt.Fprintf(os.Stdout, "Choose [%s]: ", defaultAuthMethod)
	authMethodInput, _ := reader.ReadString('\n')
	authMethodInput = strings.TrimSpace(authMethodInput)
	authMethod := defaultAuthMethod
	if authMethodInput != "" {
		authMethod = authMethodInput
	}

	var authKey, authSecret, authToken string

	if authMethod == "1" {
		// API Key + Secret
		authKeyDefault := existingConfig["auth-key"]
		if authKeyDefault != "" {
			authKeyDefault = "****" // Mask existing key
		}
		fmt.Fprintf(os.Stdout, "API Key [%s]: ", authKeyDefault)
		authKeyInput, _ := reader.ReadString('\n')
		authKeyInput = strings.TrimSpace(authKeyInput)
		if authKeyInput != "" {
			authKey = authKeyInput
		} else if existingConfig["auth-key"] != "" {
			authKey = existingConfig["auth-key"]
		}

		authSecretDefault := existingConfig["auth-secret"]
		if authSecretDefault != "" {
			authSecretDefault = "****" // Mask existing secret
		}
		fmt.Fprintf(os.Stdout, "API Secret [%s]: ", authSecretDefault)
		authSecretInput, _ := reader.ReadString('\n')
		authSecretInput = strings.TrimSpace(authSecretInput)
		if authSecretInput != "" {
			authSecret = authSecretInput
		} else if existingConfig["auth-secret"] != "" {
			authSecret = existingConfig["auth-secret"]
		}
	} else {
		// Auth Token
		authTokenDefault := existingConfig["auth-token"]
		if authTokenDefault != "" {
			authTokenDefault = "****" // Mask existing token
		}
		fmt.Fprintf(os.Stdout, "Auth Token [%s]: ", authTokenDefault)
		authTokenInput, _ := ReadLineRaw(8192)
		fmt.Println()
		authTokenInput = strings.TrimSpace(authTokenInput)
		if authTokenInput != "" {
			authToken = authTokenInput
		} else if existingConfig["auth-token"] != "" {
			authToken = existingConfig["auth-token"]
		}
	}

	// Build config data
	configData := make(map[string]interface{})
	configData["server"] = server
	configData["server-type"] = serverType
	configData["template-url"] = templateURL

	if authKey != "" {
		configData["auth-key"] = authKey
	}
	if authSecret != "" {
		configData["auth-secret"] = authSecret
	}
	if authToken != "" {
		configData["auth-token"] = authToken
	}

	// Preserve cached token fields if they exist in the current config
	if cachedToken, ok := existingConfig["cached-token"]; ok && cachedToken != "" {
		configData["cached-token"] = cachedToken
	}
	if cachedExpiry, ok := existingConfig["cached-token-expiry"]; ok {
		configData["cached-token-expiry"] = cachedExpiry
	}

	data, err := yaml.Marshal(configData)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

var ErrTooLong = errors.New("input exceeds limit")

// ReadLineRaw reads from stdin in raw mode until a newline is entered
// (accepts both '\n' and '\r'). It supports arbitrarily long lines up to `limit`.
// The terminal state is always restored before returning.
func ReadLineRaw(limit int) (string, error) {
	fd := int(os.Stdin.Fd())

	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", err
	}
	defer term.Restore(fd, oldState)

	var out bytes.Buffer
	tmp := make([]byte, 4096)

	for {
		n, rerr := os.Stdin.Read(tmp)
		if n > 0 {
			for _, c := range tmp[:n] {
				// newline pressed in raw mode is usually '\r', but handle both
				if c == '\n' || c == '\r' {
					return out.String(), nil
				}
				if out.Len() >= limit {
					// Drain until newline so the next read starts clean
					// (best-effort; ignore errors while draining)
					for c != '\n' && c != '\r' {
						var b [1]byte
						_, _ = os.Stdin.Read(b[:])
						c = b[0]
					}
					return "", ErrTooLong
				}
				out.WriteByte(c)
			}
		}
		if rerr != nil {
			if rerr == io.EOF {
				return out.String(), nil
			}
			return "", rerr
		}
	}
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)
}

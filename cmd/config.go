package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "CLI configuration management",
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

		fmt.Fprintf(os.Stderr, "✓ Configuration saved to ~/.conductor-cli/%s\n", configFileName)
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
			fmt.Fprintf(os.Stderr, "Are you sure you want to delete %s? [y/N]: ", configPath)
			response, err := reader.ReadString('\n')
			if err != nil {
				return err
			}
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Fprintf(os.Stderr, "Deletion cancelled\n")
				return nil
			}
		}

		// Delete the file
		if err := os.Remove(configPath); err != nil {
			return fmt.Errorf("failed to delete config file: %w", err)
		}

		fmt.Fprintf(os.Stderr, "✓ Configuration deleted: %s\n", configPath)
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

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
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
	fmt.Fprintf(os.Stderr, "Server URL [%s]: ", serverDefault)
	serverInput, _ := reader.ReadString('\n')
	serverInput = strings.TrimSpace(serverInput)
	server := serverDefault
	if serverInput != "" {
		server = serverInput
	}

	// Prompt for server type
	serverTypeDefault := existingConfig["server-type"]
	if serverTypeDefault == "" {
		serverTypeDefault = "OSS"
	}
	fmt.Fprintf(os.Stderr, "Server type (OSS/Enterprise) [%s]: ", serverTypeDefault)
	serverTypeInput, _ := reader.ReadString('\n')
	serverTypeInput = strings.TrimSpace(serverTypeInput)
	serverType := serverTypeDefault
	if serverTypeInput != "" {
		serverType = serverTypeInput
	}

	// Prompt for auth method
	fmt.Fprintf(os.Stderr, "\nAuthentication method:\n")
	fmt.Fprintf(os.Stderr, "  1. API Key + Secret\n")
	fmt.Fprintf(os.Stderr, "  2. Auth Token\n")

	// Determine default auth method based on existing config
	defaultAuthMethod := "1"
	if existingConfig["auth-token"] != "" {
		defaultAuthMethod = "2"
	}

	fmt.Fprintf(os.Stderr, "Choose [%s]: ", defaultAuthMethod)
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
		fmt.Fprintf(os.Stderr, "API Key [%s]: ", authKeyDefault)
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
		fmt.Fprintf(os.Stderr, "API Secret [%s]: ", authSecretDefault)
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
		fmt.Fprintf(os.Stderr, "Auth Token [%s]: ", authTokenDefault)
		authTokenInput, _ := reader.ReadString('\n')
		authTokenInput = strings.TrimSpace(authTokenInput)
		if authTokenInput != "" {
			authToken = authTokenInput
		} else if existingConfig["auth-token"] != "" {
			authToken = existingConfig["auth-token"]
		}
	}

	// Build config data
	configData := make(map[string]interface{})

	// Always write server URL
	if server != "" {
		configData["server"] = server
	}
	// Always write server type
	if serverType != "" {
		configData["server-type"] = serverType
	}
	if authKey != "" {
		configData["auth-key"] = authKey
	}
	if authSecret != "" {
		configData["auth-secret"] = authSecret
	}
	if authToken != "" {
		configData["auth-token"] = authToken
	}

	// Marshal to YAML
	data, err := yaml.Marshal(configData)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(configPath, data, 0600) // 0600 for security (credentials)
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)
}

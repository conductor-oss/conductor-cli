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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/term"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/conductor-oss/conductor-cli/internal/cliconfig"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Flags for `config show`.
var (
	configShowJSON    bool
	configShowSecrets bool
)

var configCmd = &cobra.Command{
	Use:     "config",
	Short:   "CLI configuration management",
	GroupID: "config",
}

var configSaveCmd = &cobra.Command{
	Use:   "save",
	Short: "Save configuration to file (interactive)",
	Long: `Interactively configure and save server and authentication settings.

Named profiles are saved to ~/.conductor-cli/config-<profile>.yaml. The default
configuration is saved to ~/.conductor-cli/config.yaml — leave the profile name
empty at the prompt, or pass --profile default.

If a configuration already exists, you can press Enter to keep existing values.

Examples:
  # Save the default configuration (press Enter at the profile prompt)
  conductor config save

  # Same thing, without the prompt
  conductor config save --profile default

  # Save to a named profile
  conductor config save --profile production
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := profile

		// An empty answer means the default config rather than an error (#98).
		if profileName == "" && !cmd.Flags().Changed("profile") {
			reader := bufio.NewReader(os.Stdin)
			fmt.Fprintf(os.Stdout, "Profile name (empty for default): ")
			input, _ := reader.ReadString('\n')
			profileName = strings.TrimSpace(input)
		}

		configPath, err := interactiveSaveConfig(profileName)
		if err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Fprintf(os.Stdout, "✓ Configuration saved to %s\n", configPath)
		return nil
	},
	SilenceUsage: true,
}

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration profiles",
	Long: `List all configuration profiles in ~/.conductor-cli directory.

Shows the default configuration (config.yaml) as "default", plus every named
profile (config-<profile>.yaml).

Examples:
  # List all config profiles
  conductor config list
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := cliconfig.Dir()
		if err != nil {
			return err
		}

		// Check if config directory exists
		if _, err := os.Stat(configDir); os.IsNotExist(err) {
			fmt.Println("No configuration files found")
			return nil
		}

		warnStrayDefaultFile(configDir)

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
				// Unreachable: "default" refers to config.yaml. Listing it would
				// print "default" twice. warnStrayDefaultFile explains it.
				if cliconfig.IsDefault(profileName) {
					continue
				}
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
	Long: `Delete a configuration profile.

Profile can be specified either as a positional argument or via --profile flag.

Examples:
  # Delete a named profile using positional argument
  conductor config delete production

  # Delete a named profile using --profile flag
  conductor config delete --profile production

  # Delete without confirmation
  conductor config delete production -y
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configDir, err := cliconfig.Dir()
		if err != nil {
			return err
		}

		// Get profile name from either positional arg or --profile flag
		var profileName string
		if len(args) > 0 {
			profileName = args[0]
		} else if profile != "" {
			profileName = profile
		}

		// Deleting always names its target; an empty name is too easy to hit by
		// accident.
		if profileName == "" {
			return fmt.Errorf("profile name is required (use positional argument or --profile flag, or 'default' for the default config)")
		}

		configPath := cliconfig.Resolve(configDir, profileName)

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

// interactiveSaveConfig prompts for settings and writes profileName's config
// file, returning the path. An empty name, like "default", writes config.yaml.
func interactiveSaveConfig(profileName string) (string, error) {
	configDir, err := cliconfig.Dir()
	if err != nil {
		return "", err
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	configPath := cliconfig.Resolve(configDir, profileName)

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
		serverTypeDefault = "OSS"
	}
	fmt.Fprintf(os.Stdout, "Server type (OSS/Enterprise) [%s]: ", serverTypeDefault)
	serverTypeInput, _ := reader.ReadString('\n')
	serverTypeInput = strings.TrimSpace(serverTypeInput)
	serverType := serverTypeDefault
	if serverTypeInput != "" {
		serverType = serverTypeInput
	}

	var authKey, authSecret, authToken string

	// Only prompt for authentication when using Enterprise server
	if strings.EqualFold(serverType, "OSS") {
		fmt.Fprintf(os.Stdout, "\nNo authentication required for OSS Conductor.\n")
	} else {
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
	}

	// Build config data
	configData := make(map[string]interface{})
	configData["server"] = server
	configData["server-type"] = serverType

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
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return "", err
	}
	return configPath, nil
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

// warnStrayDefaultFile reports a config-default.yaml, which no longer loads.
// Ignoring one silently leaves the user believing its settings are active.
func warnStrayDefaultFile(configDir string) {
	if stray := cliconfig.StrayDefaultFile(configDir); stray != "" {
		fmt.Fprintf(os.Stderr,
			"Warning: %s is not used; \"default\" refers to config.yaml. Rename or delete it to silence this warning.\n",
			stray)
	}
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the effective configuration and where each value came from",
	Long: `Show the configuration the CLI actually resolved, with the origin of each value.

Command-line flags override individual settings. Everything else comes from a
single source, never a mix of two:

  --config <path> or --profile <name>   that file; environment ignored
  any CONDUCTOR_* variable set          the environment; the config file is not read
  otherwise                             ~/.conductor-cli/config.yaml

Secrets are masked unless --show-secrets is passed.

Examples:
  # Show the effective configuration
  conductor config show

  # Show what a named profile resolves to
  conductor --profile production config show

  # Machine-readable output
  conductor config show --json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if configDir, err := cliconfig.Dir(); err == nil {
			warnStrayDefaultFile(configDir)
		}

		res := activeResolution(cmd)

		type entry struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			Source string `json:"source"`
		}
		entries := make([]entry, 0, len(cliconfig.Keys))
		for _, key := range cliconfig.Keys {
			value := viper.GetString(key)
			if !configShowSecrets {
				value = cliconfig.Mask(key, value)
			}
			entries = append(entries, entry{Key: key, Value: value, Source: res.SourceOf(key).ShortString()})
		}

		if configShowJSON {
			payload := struct {
				Profile string  `json:"profile"`
				File    string  `json:"file"`
				Values  []entry `json:"values"`
			}{Profile: res.Profile, File: res.File, Values: entries}
			out, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(out))
			return nil
		}

		// Only the active source is named. Printing a profile alongside
		// "environment variables" contradicts itself, because the profile
		// selects a file the CLI did not read.
		switch {
		case res.EnvBound:
			fmt.Printf("Source: environment variables\n")
		case res.File != "":
			fmt.Printf("Source: %s\n", res.File)
		default:
			fmt.Printf("Source: built-in defaults\n")
		}
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "KEY\tVALUE\tSOURCE")
		for _, e := range entries {
			value := e.Value
			if value == "" {
				value = "-"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\n", e.Key, value, e.Source)
		}
		return w.Flush()
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)
	configCmd.AddCommand(configShowCmd)

	configShowCmd.Flags().BoolVar(&configShowJSON, "json", false, "Output as JSON")
	configShowCmd.Flags().BoolVar(&configShowSecrets, "show-secrets", false, "Display secret values instead of masking them")
}

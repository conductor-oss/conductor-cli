package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSaveCmd)
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configDeleteCmd)
}

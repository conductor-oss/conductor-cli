package cmd

import (
	"context"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"strings"

	"github.com/conductor-sdk/conductor-go/sdk/client"
	sdklog "github.com/conductor-sdk/conductor-go/sdk/log"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/orkes-io/conductor-cli/internal"
	"github.com/orkes-io/conductor-cli/internal/updater"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// Version information - set via ldflags at build time
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var NAME = "orkes"

var (
	cfgFile    string
	profile    string
	url        string
	key        string
	secret     string
	token      string
	verbose    bool
	yes        bool
	serverType string
)

// confirmDeletion prompts user for confirmation unless --yes flag is set
// Returns true if user confirms or --yes is set, false otherwise
func confirmDeletion(resourceType, resourceName string) bool {
	if yes {
		return true
	}

	fmt.Printf("Are you sure you want to delete %s '%s'? (y/N): ", resourceType, resourceName)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// isEnterpriseServer checks if the configured server type is Enterprise
func isEnterpriseServer() bool {
	return strings.ToUpper(serverType) == "ENTERPRISE"
}

var rootCmd = &cobra.Command{
	Use:     NAME,
	Short:   "orkes",
	Long:    "CLI for Conductor",
	Version: fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, Date),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Suppress debug logs from conductor-go SDK at runtime
		stdlog.SetOutput(io.Discard)

		if verbose {
			log.SetLevel(log.DebugLevel)
		}

		// Check for updates if 24h have passed (non-blocking with 3s timeout)
		// Skip update check for the update command itself
		if cmd.Name() != "update" {
			updater.CheckAndUpdateState(cmd.Context(), Version)

			// Show notification if update is available
			if shouldNotify, latestVersion := updater.ShouldNotifyUpdate(Version); shouldNotify {
				fmt.Fprintf(os.Stderr, "\n⚠ A new version is available: %s (current: %s)\n", latestVersion, Version)
				fmt.Fprintf(os.Stderr, "Run 'orkes update' to download it or update with your package manager.\n\n")
			}
		}

		// Get configuration values from Viper (which handles flags, env vars, and config file)
		url = viper.GetString("server")
		key = viper.GetString("auth-key")
		secret = viper.GetString("auth-secret")
		token = viper.GetString("auth-token")
		serverType = viper.GetString("server-type")

		// Set default server type if not provided
		if serverType == "" {
			serverType = "Enterprise"
		}

		// Set default URL if not provided
		if url == "" {
			url = "http://localhost:8080/api"
		}

		// Ensure URL has /api suffix for SDK
		url = strings.TrimSuffix(url, "/")
		if !strings.HasSuffix(url, "/api") {
			url = url + "/api"
		}

		log.Debug("Using Server ", url)

		var apiClient *client.APIClient

		// Priority: auth-token > auth-key/secret
		if token != "" {
			if err := validateUserToken(token); err != nil {
				return err
			}

			tokenManager := ConfigTokenManager{
				Token: token,
			}
			apiClient = client.NewAPIClientWithTokenManager(
				nil,
				settings.NewHttpSettings(url),
				nil,
				tokenManager,
			)
		} else if key != "" && secret != "" {
			cachedToken := viper.GetString("cached-token")
			cachedExpiry := viper.GetInt64("cached-token-expiry")

			// Determine config path for saving cached token
			activeProfile := profile
			if activeProfile == "" {
				activeProfile = os.Getenv("ORKES_PROFILE")
			}

			configPath, err := getConfigPath(activeProfile)
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			tokenManager := NewCachedTokenManager(
				key,
				secret,
				cachedToken,
				cachedExpiry,
				configPath,
				settings.NewHttpSettings(url),
			)

			apiClient = client.NewAPIClientWithTokenManager(
				nil,
				settings.NewHttpSettings(url),
				nil,
				tokenManager,
			)
		} else {
			// No authentication configured, create client without credentials
			apiClient = client.NewAPIClient(
				settings.NewAuthenticationSettings("", ""),
				settings.NewHttpSettings(url),
			)
		}

		internal.SetAPIClient(apiClient)

		return nil
	},
}

func Execute(ctx context.Context) {
	cc.Init(&cc.Config{
		RootCmd:         rootCmd,
		Headings:        cc.HiWhite + cc.Bold,
		Commands:        cc.HiBlue + cc.Bold,
		Example:         cc.White,
		ExecName:        cc.Bold,
		Flags:           cc.Bold,
		NoExtraNewlines: true,
	})

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Use config directory structure: ~/.conductor-cli/config.yaml or config-<profile>.yaml
		configDir := filepath.Join(home, ".conductor-cli")
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")

		// Determine which profile to use: --profile flag takes precedence over ORKES_PROFILE env var
		activeProfile := profile
		if activeProfile == "" {
			activeProfile = os.Getenv("ORKES_PROFILE")
		}

		// Use profile-specific config if profile is set
		if activeProfile != "" {
			configName := fmt.Sprintf("config-%s", activeProfile)
			configPath := filepath.Join(configDir, configName+".yaml")

			// Check if profile config exists (skip check if we're saving a new config)
			isSavingConfig := false
			for i, arg := range os.Args {
				if arg == "config" && i+1 < len(os.Args) && os.Args[i+1] == "save" {
					isSavingConfig = true
					break
				}
			}

			if !isSavingConfig {
				if _, err := os.Stat(configPath); os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error: Profile '%s' doesn't exist (expected file: %s)\n", activeProfile, configPath)
					os.Exit(1)
				}
			}

			viper.SetConfigName(configName)
		} else {
			viper.SetConfigName("config")
		}
	}

	// Environment variable mapping
	viper.SetEnvPrefix("CONDUCTOR")
	viper.AutomaticEnv()

	// Map environment variables to config keys
	viper.BindEnv("server", "CONDUCTOR_SERVER_URL")
	viper.BindEnv("auth-key", "CONDUCTOR_AUTH_KEY")
	viper.BindEnv("auth-secret", "CONDUCTOR_AUTH_SECRET")
	viper.BindEnv("auth-token", "CONDUCTOR_AUTH_TOKEN")
	viper.BindEnv("server-type", "CONDUCTOR_SERVER_TYPE")

	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stdout, "Using config file: %s\n", viper.ConfigFileUsed())
		}
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Suppress debug logs from conductor-go SDK
	stdlog.SetOutput(io.Discard)
	stdlog.SetFlags(0)

	// Disable conductor-go SDK logging by using the noop logger
	sdklog.SetLogger(sdklog.NewNop())

	// Add command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: "conductor", Title: "Conductor Management:"},
		&cobra.Group{ID: "config", Title: "CLI Configuration:"},
		&cobra.Group{ID: "development", Title: "Development:"},
	)

	// Set group ID for auto-generated completion command
	rootCmd.SetCompletionCommandGroupID("config")

	defaultHelpFunc := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd.HasParent() {
			cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
				flag.Hidden = true
			})
		}
		defaultHelpFunc(cmd, args)
	})

	defaultUsageFunc := rootCmd.UsageFunc()
	rootCmd.SetUsageFunc(func(cmd *cobra.Command) error {
		if cmd.HasParent() {
			cmd.InheritedFlags().VisitAll(func(flag *pflag.Flag) {
				flag.Hidden = true
			})
		}
		return defaultUsageFunc(cmd)
	})

	// Configuration file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.conductor-cli/config.yaml)")

	// Server and authentication flags
	rootCmd.PersistentFlags().String("server", "", "Conductor server URL (can also be set via CONDUCTOR_SERVER_URL)")
	rootCmd.PersistentFlags().String("auth-key", "", "API key for authentication (can also be set via CONDUCTOR_AUTH_KEY)")
	rootCmd.PersistentFlags().String("auth-secret", "", "API secret for authentication (can also be set via CONDUCTOR_AUTH_SECRET)")
	rootCmd.PersistentFlags().String("auth-token", "", "Auth token for authentication (can also be set via CONDUCTOR_AUTH_TOKEN)")
	rootCmd.PersistentFlags().String("server-type", "OSS", "Server type: OSS or Enterprise (can also be set via CONDUCTOR_SERVER_TYPE)")

	// Profile and config management flags
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "use a specific profile (loads config-<profile>.yaml, can also be set via ORKES_PROFILE)")

	// Other flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print verbose logs")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "confirm yes")

	// Bind flags to viper
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("auth-key", rootCmd.PersistentFlags().Lookup("auth-key"))
	viper.BindPFlag("auth-secret", rootCmd.PersistentFlags().Lookup("auth-secret"))
	viper.BindPFlag("auth-token", rootCmd.PersistentFlags().Lookup("auth-token"))
	viper.BindPFlag("server-type", rootCmd.PersistentFlags().Lookup("server-type"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Mark mutually exclusive flags
	rootCmd.MarkFlagsMutuallyExclusive("auth-key", "auth-token")
	rootCmd.MarkFlagsMutuallyExclusive("auth-secret", "auth-token")
}

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
	"context"
	"fmt"
	"io"
	stdlog "log"
	"os"
	"strings"

	"github.com/conductor-oss/conductor-cli/internal"
	"github.com/conductor-oss/conductor-cli/internal/cliconfig"
	"github.com/conductor-oss/conductor-cli/internal/transport"
	"github.com/conductor-oss/conductor-cli/internal/updater"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	sdklog "github.com/conductor-sdk/conductor-go/sdk/log"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	cc "github.com/ivanpirog/coloredcobra"
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

var NAME = "conductor"

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

// localOnlyCommands are the top-level commands that run entirely on the local
// machine and therefore need no API client (and no server configuration).
var localOnlyCommands = map[string]bool{
	"config": true,
	"server": true,
	"update": true,
}

// isLocalOnlyCommand reports whether cmd belongs to a local-only command tree.
// Matching is anchored to the top-level command so that same-named subcommands
// elsewhere (e.g. "schedule update") still get an API client.
func isLocalOnlyCommand(cmd *cobra.Command) bool {
	topLevel := cmd
	for topLevel.Parent() != nil && topLevel.Parent().Parent() != nil {
		topLevel = topLevel.Parent()
	}
	if topLevel.Parent() == nil {
		return false // cmd is the root command itself
	}
	return localOnlyCommands[topLevel.Name()]
}

var rootCmd = &cobra.Command{
	Use:     NAME,
	Short:   "conductor",
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
				fmt.Fprintf(os.Stderr, "Run 'conductor update' to download it or update with your package manager.\n\n")
			}
		}

		// Skip API client setup for local-only commands
		if isLocalOnlyCommand(cmd) {
			return nil
		}

		// Get configuration values from Viper (which handles flags, env vars, and config file)
		url = viper.GetString("server")
		key = viper.GetString("auth-key")
		secret = viper.GetString("auth-secret")
		token = viper.GetString("auth-token")
		serverType = viper.GetString("server-type")

		// Set default server type if not provided
		if serverType == "" {
			serverType = "OSS"
		}

		// Auto-detect server if not configured
		if url == "" {
			detectedURL, err := detectOrPromptServer()
			if err != nil {
				return err
			}
			url = detectedURL
		}

		// Ensure URL has /api suffix for SDK
		url = strings.TrimSuffix(url, "/")
		if !strings.HasSuffix(url, "/api") {
			url = url + "/api"
		}

		log.Debug("Using Server ", url)

		httpSettings := settings.NewHttpSettings(url)

		var apiClient *client.APIClient
		var agentTokens transport.TokenProvider

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
				httpSettings,
				nil,
				tokenManager,
			)
			agentTokens = newTokenProvider(tokenManager, httpSettings)
		} else if key != "" && secret != "" {
			cachedToken := viper.GetString("cached-token")
			cachedExpiry := viper.GetInt64("cached-token-expiry")

			// initConfig already resolved the profile; reuse it.
			configPath, err := getConfigPath(activeProfile)
			if err != nil {
				return fmt.Errorf("failed to get config path: %w", err)
			}

			// Only cache into the default config if it already exists.
			// Creating it would plant a config.yaml on users running from the
			// environment.
			if cliconfig.IsDefault(activeProfile) {
				if _, statErr := os.Stat(configPath); statErr != nil {
					configPath = ""
				}
			}

			tokenManager := NewCachedTokenManager(
				key,
				secret,
				cachedToken,
				cachedExpiry,
				configPath,
				httpSettings,
			)

			apiClient = client.NewAPIClientWithTokenManager(
				nil,
				httpSettings,
				nil,
				tokenManager,
			)
			agentTokens = newTokenProvider(tokenManager, httpSettings)
		} else {
			// No authentication configured, create client without credentials
			apiClient = client.NewAPIClient(
				settings.NewAuthenticationSettings("", ""),
				httpSettings,
			)
		}

		internal.SetAPIClient(apiClient)

		// Share the same server URL and auth with the agent transport, whose
		// endpoints are not part of the conductor-go SDK. agentTokens is nil when no
		// credentials are configured (anonymous access).
		internal.SetTransport(transport.Config{
			BaseURL: url,
			Tokens:  agentTokens,
		})

		return nil
	},
}

func Execute(ctx context.Context) {
	cc.Init(&cc.Config{
		RootCmd:         rootCmd,
		Headings:        cc.Red + cc.Bold,
		Commands:        cc.Red + cc.Bold,
		CmdShortDescr:   cc.None,
		ExecName:        cc.Red + cc.Bold,
		Flags:           cc.Red + cc.Bold,
		FlagsDataType:   cc.Red,
		FlagsDescr:      cc.None,
		Aliases:         cc.Red + cc.Bold,
		Example:         cc.Italic,
		NoExtraNewlines: true,
	})

	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

// activeProfile is the profile initConfig settled on: --profile, else
// CONDUCTOR_PROFILE, else "" for the default config.
var activeProfile string

// loadedConfigFile is the file initConfig read, or "" when none was.
var loadedConfigFile string

// effectiveProfile prefers the --profile flag over CONDUCTOR_PROFILE.
func effectiveProfile() string {
	if profile != "" {
		return profile
	}
	return os.Getenv("CONDUCTOR_PROFILE")
}

// isSavingConfig reports whether this is `config save`, which may name a profile
// whose file does not exist yet.
func isSavingConfig() bool {
	for i, arg := range os.Args {
		if arg == "config" && i+1 < len(os.Args) && os.Args[i+1] == "save" {
			return true
		}
	}
	return false
}

// envIsActiveSource is true when the environment supplied the config, meaning no
// file was read.
var envIsActiveSource bool

func initConfig() {
	activeProfile = effectiveProfile()

	// Naming a file (--config or --profile) beats the ambient environment.
	namedFile := cfgFile != "" || !cliconfig.IsDefault(activeProfile)

	// Below flags, exactly one source supplies config: a named file, else the
	// environment if any variable is set, else config.yaml. They never merge — a
	// blended value leaves no answer to "where did this come from".
	envIsActiveSource = !namedFile && cliconfig.EnvActive()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		configDir, err := cliconfig.Dir()
		cobra.CheckErr(err)

		// Use config directory structure: ~/.conductor-cli/config.yaml or config-<profile>.yaml
		viper.AddConfigPath(configDir)
		viper.SetConfigType("yaml")

		// A named profile must already exist; the default config is optional.
		if !cliconfig.IsDefault(activeProfile) && !isSavingConfig() {
			configPath := cliconfig.Resolve(configDir, activeProfile)
			if _, err := os.Stat(configPath); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Error: Profile '%s' doesn't exist (expected file: %s)\n", activeProfile, configPath)
				os.Exit(1)
			}
		}

		// Name the file explicitly, including "config" for the default. Leaning
		// on viper's identical built-in default is how the default config kept
		// loading after config save lost the ability to write it (#98).
		viper.SetConfigName(cliconfig.FileName(activeProfile))
	}

	if envIsActiveSource {
		viper.SetEnvPrefix("CONDUCTOR")
		viper.AutomaticEnv()

		viper.BindEnv("server", "CONDUCTOR_SERVER_URL")
		viper.BindEnv("auth-key", "CONDUCTOR_AUTH_KEY")
		viper.BindEnv("auth-secret", "CONDUCTOR_AUTH_SECRET")
		viper.BindEnv("auth-token", "CONDUCTOR_AUTH_TOKEN")
		viper.BindEnv("server-type", "CONDUCTOR_SERVER_TYPE")
	} else {
		// A missing file is not an error; only a file that was read is a source.
		if err := viper.ReadInConfig(); err == nil {
			loadedConfigFile = viper.ConfigFileUsed()
		}
	}

	if viper.GetBool("verbose") {
		reportConfigSources()
	}
}

// reportConfigSources prints where each setting came from.
func reportConfigSources() {
	res := cliconfig.NewResolution(activeProfile, loadedConfigFile, envIsActiveSource, rootFlagChanged)

	switch {
	case res.EnvBound:
		fmt.Fprintf(os.Stdout, "Using environment variables (config file not read)\n")
	case res.File != "":
		fmt.Fprintf(os.Stdout, "Using config file: %s\n", res.File)
	}
	for _, key := range cliconfig.Keys {
		src := res.SourceOf(key)
		if src.Kind == cliconfig.SourceDefault {
			continue
		}
		// The full path is already on the line above.
		fmt.Fprintf(os.Stdout, "Using %s: %s (%s)\n", key, cliconfig.Mask(key, viper.GetString(key)), src.ShortString())
	}
}

// rootFlagChanged reports whether the user passed the flag for key.
func rootFlagChanged(key string) bool {
	f := rootCmd.PersistentFlags().Lookup(key)
	return f != nil && f.Changed
}

// activeResolution describes what this invocation resolved, including the flags
// cmd saw.
func activeResolution(cmd *cobra.Command) cliconfig.Resolution {
	return cliconfig.NewResolution(activeProfile, loadedConfigFile, envIsActiveSource, func(key string) bool {
		f := cmd.Flags().Lookup(key)
		return f != nil && f.Changed
	})
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (overrides profile-based config loading)")

	// Server and authentication flags
	rootCmd.PersistentFlags().String("server", "", "Conductor server URL (can also be set via CONDUCTOR_SERVER_URL)")
	rootCmd.PersistentFlags().String("auth-key", "", "API key for authentication (can also be set via CONDUCTOR_AUTH_KEY)")
	rootCmd.PersistentFlags().String("auth-secret", "", "API secret for authentication (can also be set via CONDUCTOR_AUTH_SECRET)")
	rootCmd.PersistentFlags().String("auth-token", "", "Auth token for authentication (can also be set via CONDUCTOR_AUTH_TOKEN)")
	rootCmd.PersistentFlags().String("server-type", "OSS", "Server type: OSS or Enterprise (can also be set via CONDUCTOR_SERVER_TYPE)")

	// Profile and config management flags
	rootCmd.PersistentFlags().StringVar(&profile, "profile", "", "use a specific profile (loads config-<profile>.yaml, can also be set via CONDUCTOR_PROFILE)")

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

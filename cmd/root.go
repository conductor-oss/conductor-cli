package cmd

import (
	"context"
	"fmt"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	sdklog "github.com/conductor-sdk/conductor-go/sdk/log"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"os"
	"time"
)

var (
	// Version information - set via ldflags at build time
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

var NAME = "orkes"

var (
	cfgFile string
	url     string
	key     string
	secret  string
	token   string
	verbose bool
	yes     bool
)
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
		// Get configuration values from Viper (which handles flags, env vars, and config file)
		url = viper.GetString("server")
		key = viper.GetString("auth-key")
		secret = viper.GetString("auth-secret")
		token = viper.GetString("auth-token")

		// Set default URL if not provided
		if url == "" {
			url = "http://localhost:8080/api"
		}

		log.Debug("Using Server ", url)
		apiClient := client.NewAPIClient(settings.NewAuthenticationSettings(key, secret), settings.NewHttpSettings(url))

		if token != "" {
			tokenManager := ConfigTokenManager{
				Token: token,
			}
			apiClient = client.NewAPIClientWithTokenManager(
				nil,
				settings.NewHttpSettings(url),
				nil,
				tokenManager,
			)
		}

		internal.SetAPIClient(apiClient)

		return nil
	},
}

func Execute(ctx context.Context) {
	err := rootCmd.ExecuteContext(ctx)
	if err != nil {
		os.Exit(1)
	}
}

func getHttpClient() *http.Client {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	netTransport := &http.Transport{
		DialContext:         baseDialer.DialContext,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		DisableCompression:  false,
	}
	client := http.Client{
		Transport:     netTransport,
		CheckRedirect: nil,
		Jar:           nil,
		Timeout:       30 * time.Second,
	}
	return &client

}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".conductor-cli")
	}

	// Environment variable mapping
	viper.SetEnvPrefix("CONDUCTOR")
	viper.AutomaticEnv()

	// Map environment variables to config keys
	viper.BindEnv("server", "CONDUCTOR_SERVER_URL")
	viper.BindEnv("auth-key", "CONDUCTOR_AUTH_KEY")
	viper.BindEnv("auth-secret", "CONDUCTOR_AUTH_SECRET")
	viper.BindEnv("auth-token", "CONDUCTOR_AUTH_TOKEN")

	if err := viper.ReadInConfig(); err == nil {
		if viper.GetBool("verbose") {
			fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
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

	// Configuration file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.conductor-cli.yaml)")

	// Server and authentication flags
	rootCmd.PersistentFlags().String("server", "", "Conductor server URL (can also be set via CONDUCTOR_SERVER_URL)")
	rootCmd.PersistentFlags().String("auth-key", "", "API key for authentication (can also be set via CONDUCTOR_AUTH_KEY)")
	rootCmd.PersistentFlags().String("auth-secret", "", "API secret for authentication (can also be set via CONDUCTOR_AUTH_SECRET)")
	rootCmd.PersistentFlags().String("auth-token", "", "Auth token for authentication (can also be set via CONDUCTOR_AUTH_TOKEN)")

	// Other flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print verbose logs")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "confirm yes")

	// Bind flags to viper
	viper.BindPFlag("server", rootCmd.PersistentFlags().Lookup("server"))
	viper.BindPFlag("auth-key", rootCmd.PersistentFlags().Lookup("auth-key"))
	viper.BindPFlag("auth-secret", rootCmd.PersistentFlags().Lookup("auth-secret"))
	viper.BindPFlag("auth-token", rootCmd.PersistentFlags().Lookup("auth-token"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Mark mutually exclusive flags
	rootCmd.MarkFlagsMutuallyExclusive("auth-key", "auth-token")
	rootCmd.MarkFlagsMutuallyExclusive("auth-secret", "auth-token")
}

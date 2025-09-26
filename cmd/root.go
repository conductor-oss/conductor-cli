package cmd

import (
	"context"
	"io"
	stdlog "log"
	"github.com/conductor-sdk/conductor-go/sdk/client"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	sdklog "github.com/conductor-sdk/conductor-go/sdk/log"
	"github.com/orkes-io/conductor-cli/internal"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"net"
	"net/http"
	"os"
	"time"
)

var NAME = "cdt"

var (
	url     string
	key     string
	secret  string
	token   string
	verbose bool
	yes     bool
)
var rootCmd = &cobra.Command{
	Use:   NAME,
	Short: "cdt",
	Long:  "CLI for Conductor",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Suppress debug logs from conductor-go SDK at runtime
		stdlog.SetOutput(io.Discard)

		if verbose {
			log.SetLevel(log.DebugLevel)
		}
		serverConfig := getActiveConfig()
		if serverConfig == nil {
			serverConfig = &Config{
				URL: "http://localhost:8080/api",
			}
		}
		log.Debug("Using Server ", serverConfig.URL)

		url = serverConfig.URL
		key = serverConfig.Key
		secret = serverConfig.Secret
		token = serverConfig.Token
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

func init() {
	// Suppress debug logs from conductor-go SDK
	stdlog.SetOutput(io.Discard)
	stdlog.SetFlags(0)
	
	// Disable conductor-go SDK logging by using the noop logger
	sdklog.SetLogger(sdklog.NewNop())
	
	rootCmd.PersistentFlags().StringVar(&url, "url", "", "server url")
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "auth token")
	rootCmd.PersistentFlags().StringVar(&key, "key", "", "auth key")
	rootCmd.PersistentFlags().StringVar(&secret, "secret", "", "auth secret")

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print verbose logs")
	rootCmd.MarkFlagsMutuallyExclusive("key", "token")
	rootCmd.MarkFlagsMutuallyExclusive("secret", "token")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "confirm yes")

}

package cmd

import (
	"errors"
	"github.com/conductor-sdk/conductor-go/sdk/settings"
	"net/http"
)

type ConfigTokenManager struct {
	Token string
}

func (manager ConfigTokenManager) RefreshToken(httpSettings *settings.HttpSettings, httpClient *http.Client) (string, error) {
	if manager.Token != "" {
		return manager.Token, nil
	} else {
		return "", errors.New("missing token")
	}

}

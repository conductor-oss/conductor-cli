package cmd

import (
	"os"
)

const CONDUCTOR_SERVER_URL = "CONDUCTOR_SERVER_URL"
const CONDUCTOR_AUTH_KEY = "CONDUCTOR_AUTH_KEY"
const CONDUCTOR_AUTH_SECRET = "CONDUCTOR_AUTH_SECRET"
const CONDUCTOR_AUTH_TOKEN = "CONDUCTOR_AUTH_TOKEN"

type Config struct {
	URL    string `json:url`
	Token  string `json:token`
	Key    string `json:token`
	Secret string `json:token`
}

func getActiveConfig() *Config {

	config := Config{
		URL:    os.Getenv(CONDUCTOR_SERVER_URL),
		Token:  os.Getenv(CONDUCTOR_AUTH_TOKEN),
		Key:    os.Getenv(CONDUCTOR_AUTH_KEY),
		Secret: os.Getenv(CONDUCTOR_AUTH_SECRET),
	}
	return &config
}

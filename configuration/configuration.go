// Package configuration is deprecated: Bard-era client, removed before v0.2.0. Use gogemini.New and Client.GenerateContent.
package configuration

import (
	"github.com/Allan-Nava/go-gemini/constants"
	"github.com/caarlos0/env/v6"
)

// Deprecated: Bard-era client, removed before v0.2.0. Use gogemini.New and Client.GenerateContent.
type Configuration struct {
	IsDebug    bool `env:"IS_DEBUG"`
	BaseUrl    string
	BardApiKey string `env:"_BARD_API_KEY"`
	//RestClient *resty.Client
}

// Deprecated: Bard-era client, removed before v0.2.0. Use gogemini.New and Client.GenerateContent.
func GetConfiguration() *Configuration {
	configuration := Configuration{}
	err := env.Parse(&configuration)
	if err != nil {
		panic("failed to read configuration")
	}
	//
	configuration.BaseUrl = constants.BASE_URL
	//
	return &configuration
}

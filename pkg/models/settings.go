package models

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type PluginSettings struct {
	AccountID  string                `json:"accountId"`
	DatabaseID string                `json:"databaseId"`
	Secrets    *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	APIToken string `json:"apiToken"`
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	err := json.Unmarshal(source.JSONData, &settings)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
	}

	// Initialize Secrets to avoid nil pointer dereference if DecryptedSecureJSONData is empty
	settings.Secrets = &SecretPluginSettings{}
	if source.DecryptedSecureJSONData != nil {
		settings.Secrets.APIToken = source.DecryptedSecureJSONData["apiToken"]
	}

	return &settings, nil
}

// loadSecretPluginSettings is no longer needed as logic is moved into LoadPluginSettings
// We can remove it or keep it if we anticipate more complex secret loading later.
// For now, let's comment it out to simplify.
/*
func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		APIToken: source["apiToken"], // Changed from ApiKey to APIToken and apiKey to apiToken
	}
}
*/

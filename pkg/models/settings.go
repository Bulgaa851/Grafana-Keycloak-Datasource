package models

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// PluginSettings holds the non-sensitive datasource configuration. These values
// come from `jsonData` and are safe to expose to the frontend.
type PluginSettings struct {
	BaseURL  string `json:"baseUrl"`
	Realm    string `json:"realm"`
	ClientID string `json:"clientId"`

	// Secrets holds sensitive values decrypted from secureJsonData. It is never
	// serialised back to the frontend (json:"-").
	Secrets *SecretPluginSettings `json:"-"`
}

// SecretPluginSettings holds values that live in secureJsonData and must never
// be sent to the frontend, logged, or placed into error messages.
type SecretPluginSettings struct {
	ClientSecret string `json:"clientSecret"`
}

// LoadPluginSettings unmarshals the datasource instance settings into a
// PluginSettings, including the decrypted client secret.
func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	settings := PluginSettings{}
	if err := json.Unmarshal(source.JSONData, &settings); err != nil {
		return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
	}

	// Normalise the base URL (strip a trailing slash) so path joining is simple.
	settings.BaseURL = strings.TrimRight(strings.TrimSpace(settings.BaseURL), "/")
	settings.Realm = strings.TrimSpace(settings.Realm)
	settings.ClientID = strings.TrimSpace(settings.ClientID)

	settings.Secrets = loadSecretPluginSettings(source.DecryptedSecureJSONData)

	return &settings, nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		ClientSecret: source["clientSecret"],
	}
}

// Validate returns a readable error if any required configuration field is
// missing. It never includes the secret value in the message.
func (s *PluginSettings) Validate() error {
	var missing []string
	if s.BaseURL == "" {
		missing = append(missing, "Keycloak base URL")
	}
	if s.Realm == "" {
		missing = append(missing, "Realm")
	}
	if s.ClientID == "" {
		missing = append(missing, "Client ID")
	}
	if s.Secrets == nil || s.Secrets.ClientSecret == "" {
		missing = append(missing, "Client Secret")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}
	return nil
}

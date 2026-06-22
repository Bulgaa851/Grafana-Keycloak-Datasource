package plugin

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/example/keycloak-metrics-datasource/pkg/keycloak"
	"github.com/example/keycloak-metrics-datasource/pkg/models"
)

// Make sure Datasource implements required interfaces. This is important to do
// since otherwise we will only get a not implemented error response from plugin
// in runtime.
var (
	_ backend.QueryDataHandler      = (*Datasource)(nil)
	_ backend.CheckHealthHandler    = (*Datasource)(nil)
	_ instancemgmt.InstanceDisposer = (*Datasource)(nil)
)

// Datasource is the Keycloak datasource instance. One instance exists per
// configured datasource; it owns a Keycloak client (and therefore its own
// cached access token).
type Datasource struct {
	settings *models.PluginSettings
	client   *keycloak.Client
}

// NewDatasource creates a new datasource instance. It loads settings and builds
// the Keycloak client. Invalid/incomplete settings are not fatal here: the
// concrete errors are reported by CheckHealth and QueryData so the user sees a
// readable message in the UI.
func NewDatasource(_ context.Context, dsSettings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	settings, err := models.LoadPluginSettings(dsSettings)
	if err != nil {
		return nil, err
	}
	return &Datasource{
		settings: settings,
		client:   keycloak.New(settings),
	}, nil
}

// Dispose cleans up datasource instance resources. The Keycloak client holds
// only an in-memory token, which is dropped together with the instance.
func (d *Datasource) Dispose() {}

// QueryData handles multiple queries and returns multiple responses.
func (d *Datasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	for _, q := range req.Queries {
		response.Responses[q.RefID] = d.query(ctx, q)
	}
	return response, nil
}

// CheckHealth obtains a token and then calls users/count. It returns OK only if
// both succeed; otherwise it returns a readable, secret-free error message.
func (d *Datasource) CheckHealth(ctx context.Context, _ *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	if err := d.settings.Validate(); err != nil {
		return errorHealth(err.Error()), nil
	}

	// UsersCount performs the token request (client_credentials) and then the
	// admin call, so a success here proves both steps work.
	count, err := d.client.UsersCount(ctx)
	if err != nil {
		return errorHealth(err.Error()), nil
	}

	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: messageForCount(d.settings.Realm, count),
	}, nil
}

func errorHealth(msg string) *backend.CheckHealthResult {
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusError,
		Message: msg,
	}
}

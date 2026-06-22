package plugin

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/example/keycloak-metrics-datasource/pkg/keycloak"
	"github.com/example/keycloak-metrics-datasource/pkg/models"
)

// newTestDatasource builds a Datasource with the given settings. The Keycloak
// client is real but never reached in tests that only exercise validation or
// dispatch-before-network paths.
func newTestDatasource(s *models.PluginSettings) *Datasource {
	return &Datasource{settings: s, client: keycloak.New(s)}
}

func TestQueryData_UnconfiguredReturnsError(t *testing.T) {
	ds := newTestDatasource(&models.PluginSettings{Secrets: &models.SecretPluginSettings{}})

	q, _ := json.Marshal(queryModel{QueryType: queryTypeUserCount})
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A", JSON: q}},
	})
	if err != nil {
		t.Fatalf("QueryData returned transport error: %v", err)
	}
	if got := resp.Responses["A"]; got.Error == nil {
		t.Fatal("expected an error response when datasource is not configured")
	}
}

func TestQueryData_UnknownType(t *testing.T) {
	ds := newTestDatasource(&models.PluginSettings{
		BaseURL:  "https://kc.example",
		Realm:    "demo",
		ClientID: "grafana-ds",
		Secrets:  &models.SecretPluginSettings{ClientSecret: "x"},
	})

	q, _ := json.Marshal(queryModel{QueryType: "does_not_exist"})
	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		Queries: []backend.DataQuery{{RefID: "A", JSON: q}},
	})
	if err != nil {
		t.Fatalf("QueryData returned transport error: %v", err)
	}
	if resp.Responses["A"].Error == nil {
		t.Fatal("expected an error response for an unknown query type")
	}
}

func TestBucketEvents(t *testing.T) {
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	to := from + int64(time.Hour/time.Millisecond) // 1 hour span

	events := []keycloak.Event{
		{Time: from + 10, Type: "LOGIN"},                 // first bucket
		{Time: from + 20, Type: "LOGIN"},                 // first bucket
		{Time: to - 10, Type: "LOGIN"},                   // last bucket
		{Time: from + int64(30*time.Minute/time.Millisecond)}, // middle
	}

	times, counts := bucketEvents(events, from, to)

	if len(times) != len(counts) {
		t.Fatalf("times/counts length mismatch: %d vs %d", len(times), len(counts))
	}
	var total int64
	for _, c := range counts {
		total += c
	}
	if total != int64(len(events)) {
		t.Fatalf("expected all %d events bucketed, got %d", len(events), total)
	}
	if counts[0] != 2 {
		t.Fatalf("expected 2 events in first bucket, got %d", counts[0])
	}
	if times[0].UnixMilli() != from {
		t.Fatalf("first bucket should start at 'from'")
	}
}

func TestScalarFrame(t *testing.T) {
	resp := scalarFrame("user_count", 42)
	if len(resp.Frames) != 1 {
		t.Fatalf("expected 1 frame, got %d", len(resp.Frames))
	}
	f := resp.Frames[0].Fields[0]
	if f.Len() != 1 {
		t.Fatalf("expected 1 row, got %d", f.Len())
	}
	if v, _ := f.ConcreteAt(0); v.(int64) != 42 {
		t.Fatalf("expected value 42, got %v", v)
	}
}

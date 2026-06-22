package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/example/keycloak-metrics-datasource/pkg/keycloak"
)

// Query types supported by the query editor's queryType dropdown.
const (
	queryTypeUserCount      = "user_count"
	queryTypeEventCount     = "event_count"
	queryTypeActiveSessions = "active_sessions"
	queryTypeRealmRoles     = "realm_roles"
	queryTypeGroupsCount    = "groups_count"
)

// queryModel is the JSON sent by the query editor for a single query.
type queryModel struct {
	QueryType string `json:"queryType"`
	EventType string `json:"eventType"` // used by event_count
	ClientID  string `json:"clientId"`  // used by active_sessions
}

func (d *Datasource) query(ctx context.Context, query backend.DataQuery) backend.DataResponse {
	var qm queryModel
	if err := json.Unmarshal(query.JSON, &qm); err != nil {
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("invalid query: %v", err))
	}

	// Every query needs valid connection settings.
	if err := d.settings.Validate(); err != nil {
		return backend.ErrDataResponse(backend.StatusValidationFailed, err.Error())
	}

	switch qm.QueryType {
	case queryTypeUserCount:
		return d.queryUserCount(ctx)
	case queryTypeGroupsCount:
		return d.queryGroupsCount(ctx)
	case queryTypeActiveSessions:
		return d.queryActiveSessions(ctx, qm.ClientID)
	case queryTypeRealmRoles:
		return d.queryRealmRoles(ctx)
	case queryTypeEventCount:
		return d.queryEventCount(ctx, qm.EventType, query.TimeRange)
	case "":
		return backend.ErrDataResponse(backend.StatusBadRequest, "no query type selected")
	default:
		return backend.ErrDataResponse(backend.StatusBadRequest, fmt.Sprintf("unknown query type %q", qm.QueryType))
	}
}

// ----- user_count -------------------------------------------------------------

func (d *Datasource) queryUserCount(ctx context.Context) backend.DataResponse {
	count, err := d.client.UsersCount(ctx)
	if err != nil {
		return errResponse(err)
	}
	return scalarFrame("user_count", count)
}

// ----- groups_count -----------------------------------------------------------

func (d *Datasource) queryGroupsCount(ctx context.Context) backend.DataResponse {
	count, err := d.client.GroupsCount(ctx)
	if err != nil {
		return errResponse(err)
	}
	return scalarFrame("groups_count", count)
}

// ----- active_sessions --------------------------------------------------------

func (d *Datasource) queryActiveSessions(ctx context.Context, clientID string) backend.DataResponse {
	count, err := d.client.ActiveSessions(ctx, clientID)
	if err != nil {
		return errResponse(err)
	}
	return scalarFrame("active_sessions", count)
}

// ----- realm_roles ------------------------------------------------------------

func (d *Datasource) queryRealmRoles(ctx context.Context) backend.DataResponse {
	roles, err := d.client.RealmRoles(ctx)
	if err != nil {
		return errResponse(err)
	}

	names := make([]string, len(roles))
	descriptions := make([]string, len(roles))
	composite := make([]bool, len(roles))
	ids := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
		descriptions[i] = r.Description
		composite[i] = r.Composite
		ids[i] = r.ID
	}

	frame := data.NewFrame("realm_roles",
		data.NewField("name", nil, names),
		data.NewField("description", nil, descriptions),
		data.NewField("composite", nil, composite),
		data.NewField("id", nil, ids),
	)
	frame.Meta = &data.FrameMeta{PreferredVisualization: data.VisTypeTable}

	var response backend.DataResponse
	response.Frames = append(response.Frames, frame)
	return response
}

// ----- event_count (time series) ---------------------------------------------

func (d *Datasource) queryEventCount(ctx context.Context, eventType string, tr backend.TimeRange) backend.DataResponse {
	fromMs := tr.From.UnixMilli()
	toMs := tr.To.UnixMilli()

	events, err := d.client.Events(ctx, eventType, fromMs, toMs)
	if err != nil {
		return errResponse(err)
	}

	times, counts := bucketEvents(events, fromMs, toMs)

	name := "event_count"
	if eventType != "" {
		name = eventType
	}
	frame := data.NewFrame("events",
		data.NewField("time", nil, times),
		data.NewField(name, nil, counts),
	)
	frame.Meta = &data.FrameMeta{PreferredVisualization: data.VisTypeGraph}

	var response backend.DataResponse
	response.Frames = append(response.Frames, frame)
	return response
}

// bucketEvents aggregates events into evenly spaced time buckets across the
// [fromMs, toMs] range, producing a regular time series (including empty
// buckets) suitable for a graph panel.
func bucketEvents(events []keycloak.Event, fromMs, toMs int64) ([]time.Time, []int64) {
	const targetBuckets = 50

	span := toMs - fromMs
	if span <= 0 {
		span = 1
	}
	bucketMs := span / targetBuckets
	if bucketMs < 1 {
		bucketMs = 1
	}

	numBuckets := int(span/bucketMs) + 1
	times := make([]time.Time, numBuckets)
	counts := make([]int64, numBuckets)
	for i := 0; i < numBuckets; i++ {
		times[i] = time.UnixMilli(fromMs + int64(i)*bucketMs)
	}

	for _, e := range events {
		idx := int((e.Time - fromMs) / bucketMs)
		if idx < 0 {
			idx = 0
		}
		if idx >= numBuckets {
			idx = numBuckets - 1
		}
		counts[idx]++
	}
	return times, counts
}

// ----- shared helpers ---------------------------------------------------------

// scalarFrame builds a single-value frame suitable for a Stat panel (and usable
// as a one-row table).
func scalarFrame(name string, value int64) backend.DataResponse {
	frame := data.NewFrame(name,
		data.NewField(name, nil, []int64{value}),
	)
	frame.Meta = &data.FrameMeta{PreferredVisualization: data.VisTypeTable}

	var response backend.DataResponse
	response.Frames = append(response.Frames, frame)
	return response
}

// errResponse maps a backend error to a Grafana data response, classifying
// auth/permission problems as downstream errors.
func errResponse(err error) backend.DataResponse {
	return backend.ErrDataResponse(backend.StatusInternal, err.Error())
}

func messageForCount(realm string, count int64) string {
	return fmt.Sprintf("Success: authenticated to realm %q, %d user(s) found", realm, count)
}

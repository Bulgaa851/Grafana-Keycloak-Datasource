# Keycloak data source

Visualise [Keycloak](https://www.keycloak.org/) Admin REST API metrics on your
Grafana dashboards.

A Go backend authenticates to Keycloak using the OAuth2 `client_credentials`
grant (the client secret stays server-side, in Grafana's encrypted
`secureJsonData`) and exposes these query types:

- **User count** — total users in the realm
- **Event count** — login/event activity as a time series over the dashboard range
- **Active sessions** — active sessions for a given client
- **Realm roles** — all realm roles as a table
- **Groups count** — total groups in the realm

## Requirements

- Keycloak (tested with 22.0.4) reachable from the Grafana server.
- A confidential Keycloak client with a service account and read roles
  (`view-users`, `query-users`, `view-events`, `view-realm`, `query-groups`, and
  `view-clients` for active sessions).
- Grafana OSS 11+.

## Getting started

1. Add a **Keycloak** data source.
2. Enter the base URL, realm, client ID and client secret, then **Save & test**.
3. Build a panel and choose a **Query type** in the query editor.

See the repository README for full Keycloak client-setup, build and install
instructions.

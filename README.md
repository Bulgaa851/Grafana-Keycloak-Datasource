# Keycloak data source for Grafana

A Grafana **data source plugin** (with a Go backend) that connects to the
**Keycloak Admin REST API** so you can visualise Keycloak metrics — user counts,
login/event activity, active sessions, realm roles and group counts — on Grafana
dashboards.

- **Plugin ID:** `keycloak-metrics-datasource`
- **Tested against:** Keycloak **22.0.4**, Grafana OSS **11+** (loads on 13.x)
- All Keycloak calls happen in the Go backend. The browser never talks to
  Keycloak directly, and the client secret never leaves the server.

---

## How it works

The backend authenticates to Keycloak with the OAuth2 **`client_credentials`**
grant:

```
POST {baseUrl}/realms/{realm}/protocol/openid-connect/token
     grant_type=client_credentials&client_id=...&client_secret=...
```

The returned `access_token` is cached in memory and refreshed automatically
shortly before it expires. Every admin call (e.g. `GET /admin/realms/{realm}/users/count`)
reuses the cached token and is retried once with a fresh token if Keycloak
responds `401`.

> **Security:** the client secret is stored only in Grafana's `secureJsonData`
> (encrypted at rest). It is never written to `jsonData`, never sent to the
> frontend, and never logged or included in error messages. The plugin verifies
> Keycloak TLS certificates (no insecure-skip option).

---

## 1. Keycloak setup (what an admin must configure)

> This plugin does **not** create anything in Keycloak. An administrator must
> create a confidential client with a service account and assign it read roles.

In the realm you want to monitor (repeat per realm):

1. **Create a client**
   - *Clients → Create client*
   - Client type: **OpenID Connect**
   - Client ID: e.g. `grafana-ds`
   - Next →
   - **Client authentication: On** (this makes it a *confidential* client)
   - **Service accounts roles: On** (enables the `client_credentials` grant)
   - You can turn *Standard flow* and *Direct access grants* **Off** — they are
     not needed.
   - Save.

2. **Copy the client secret**
   - *Clients → grafana-ds → Credentials → Client secret* → copy it. You will
     paste this into the Grafana data source config.

3. **Assign service-account roles**
   - *Clients → grafana-ds → Service accounts roles → Assign role*
   - Filter by **clients** and assign these **`realm-management`** roles:

     | Role            | Needed for                                   |
     | --------------- | -------------------------------------------- |
     | `view-users`    | user count, query users                      |
     | `query-users`   | user count                                   |
     | `view-events`   | event count (login/event activity)           |
     | `view-realm`    | realm roles, realm config                    |
     | `query-groups`  | groups count                                 |
     | `view-clients`  | **active sessions** (resolve client + count) |

   > The first five roles are the baseline. `view-clients` is **additionally
   > required only for the `active_sessions` query type**, because it must look
   > up the client by its `clientId` and read its session count. If you do not
   > use `active_sessions`, you can omit `view-clients`.

4. **Enable events** (only needed for the `event_count` query)
   - *Realm settings → Events → User events settings → Save events: On → Save.*
   - Optionally set which event types are saved (default is all).

---

## 2. Configure the data source in Grafana

*Connections → Data sources → Add data source → Keycloak*, then fill in:

| Field             | Stored in        | Example                     |
| ----------------- | ---------------- | --------------------------- |
| Keycloak base URL | `jsonData`       | `https://keycloak.example.com` |
| Realm             | `jsonData`       | `master`                    |
| Client ID         | `jsonData`       | `grafana-ds`                |
| Client Secret     | `secureJsonData` | *(the secret from step 2)*  |

Click **Save & test**. This triggers the backend `CheckHealth`, which obtains a
token and calls `GET /admin/realms/{realm}/users/count`. It returns OK only if
**both** the token request and the admin call succeed; otherwise it shows a
readable error (e.g. missing roles, wrong realm, bad secret).

---

## 3. Query types

Pick a **Query type** in the panel's query editor:

| Query type        | Keycloak endpoint                                              | Frame / panel        |
| ----------------- | ------------------------------------------------------------- | -------------------- |
| `user_count`      | `GET /admin/realms/{realm}/users/count`                       | single value → Stat  |
| `groups_count`    | `GET /admin/realms/{realm}/groups/count`                      | single value → Stat  |
| `realm_roles`     | `GET /admin/realms/{realm}/roles`                             | table                |
| `active_sessions` | resolve client by `clientId` → `…/clients/{id}/session-count` | single value → Stat  |
| `event_count`     | `GET /admin/realms/{realm}/events` (filtered by type + range) | **time series**      |

Extra fields appear when relevant:

- **`active_sessions`** → *Client ID*: the `clientId` whose active sessions are counted.
- **`event_count`** → *Event type* (optional, e.g. `LOGIN`; empty = all types).

`event_count` respects the **dashboard time range**: Keycloak's server-side date
filter is day-grained, so the backend requests the covering day range and then
filters precisely by the panel's epoch-millisecond `from`/`to` before bucketing
the events into an evenly-spaced time series.

---

## 4. Build

Prerequisites: Node.js 20+, Go 1.21+, and [Mage](https://magefile.org)
(`go install github.com/magefile/mage@latest`).

```bash
# Frontend
npm install
npm run build          # production bundle into dist/

# Backend (Go)
mage -v build:backend  # build for your current OS into dist/
# Cross-compile for the OS that runs Grafana, e.g. a Linux server / container:
mage -v build:linux
# Build for all platforms:
mage -v
```

Useful extras: `mage -l` (list targets), `npm run typecheck`, `npm run test:ci`
(Jest), `go test ./...` (backend tests).

---

## 5. Install into Grafana

1. Build the plugin (frontend **and** the backend binary for the Grafana host's
   OS/arch — see above).
2. Copy the whole `dist/` directory into Grafana's plugins directory, named by
   the plugin ID:

   ```
   <grafana-data>/plugins/keycloak-metrics-datasource/
   ```

   (default `/var/lib/grafana/plugins/` on Linux packages).
3. Because this plugin is **unsigned**, allow it explicitly in `grafana.ini`
   (or `custom.ini`):

   ```ini
   [plugins]
   allow_loading_unsigned_plugins = keycloak-metrics-datasource
   ```

   The equivalent environment variable is
   `GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS=keycloak-metrics-datasource`.
4. Restart Grafana. Any change to `plugin.json` also requires a restart.

---

## 6. Local development environment

The scaffolded Docker setup runs Grafana with the plugin auto-loaded and the
datasource auto-provisioned.

```bash
npm install
npm run build
mage -v build:linux     # the dev Grafana container is Linux
npm run server          # docker compose up (Grafana on http://localhost:3000)
```

Provisioning lives in [`provisioning/datasources/datasources.yml`](./provisioning/datasources/datasources.yml).
Update `baseUrl`, `realm`, `clientId` and the `secureJsonData.clientSecret`
placeholder for your environment. The dev `docker-compose.yaml` already sets
`GF_PLUGINS_ALLOW_LOADING_UNSIGNED_PLUGINS` for this plugin ID.

> If host port `3000` is already in use, change the published port in
> `docker-compose.yaml` (or stop the conflicting service) before `npm run server`.

---

## Project layout

```
src/                         Frontend (TypeScript / React)
  components/ConfigEditor.tsx   Base URL, Realm, Client ID, Client Secret
  components/QueryEditor.tsx    Query type dropdown + conditional fields
  datasource.ts, types.ts, module.ts
pkg/                         Backend (Go)
  models/settings.go            jsonData + secureJsonData loading & validation
  keycloak/client.go            token caching + Admin REST API calls
  plugin/datasource.go          CheckHealth, QueryData dispatch
  plugin/query.go               per-query-type data frames
provisioning/                Dev auto-provisioning
.config/                     Build toolchain (managed by create-plugin — do not edit)
```

---

## Troubleshooting

| Symptom (Save & test / query error)                     | Likely cause                                                            |
| ------------------------------------------------------- | ----------------------------------------------------------------------- |
| `authentication with Keycloak failed (invalid_client)`  | Wrong Client ID/secret, or *Client authentication* / service account off |
| `Keycloak denied the request … missing … roles`         | Service account lacks the required `realm-management` role              |
| `client "…" was not found in realm`                     | `active_sessions` client ID is wrong, or `view-clients` not assigned    |
| `event_count` returns nothing                           | "Save events" is disabled for the realm, or no events in the time range |
| Plugin doesn't appear in Grafana                        | Missing `allow_loading_unsigned_plugins`, or backend binary not built for the host OS |

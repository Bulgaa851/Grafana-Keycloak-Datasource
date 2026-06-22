import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

/**
 * The query types exposed by the query editor's "Query type" dropdown. Each maps
 * to a Keycloak Admin REST endpoint handled by the Go backend.
 */
export type KeycloakQueryType = 'user_count' | 'event_count' | 'active_sessions' | 'realm_roles' | 'groups_count';

export interface KeycloakQuery extends DataQuery {
  queryType: KeycloakQueryType;
  /** Optional Keycloak event type filter, used by `event_count` (e.g. LOGIN). */
  eventType?: string;
  /** Client ID whose active sessions are counted, used by `active_sessions`. */
  clientId?: string;
}

export const DEFAULT_QUERY: Partial<KeycloakQuery> = {
  queryType: 'user_count',
};

/**
 * Non-sensitive datasource options stored in jsonData (safe for the frontend).
 */
export interface KeycloakDataSourceOptions extends DataSourceJsonData {
  baseUrl?: string;
  realm?: string;
  clientId?: string;
}

/**
 * Sensitive values stored in secureJsonData. Never sent back to the frontend.
 */
export interface KeycloakSecureJsonData {
  clientSecret?: string;
}

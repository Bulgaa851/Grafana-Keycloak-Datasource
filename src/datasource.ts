import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { KeycloakQuery, KeycloakDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<KeycloakQuery, KeycloakDataSourceOptions> {
  constructor(instanceSettings: DataSourceInstanceSettings<KeycloakDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<KeycloakQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: KeycloakQuery, scopedVars: ScopedVars): KeycloakQuery {
    const templateSrv = getTemplateSrv();
    return {
      ...query,
      clientId: query.clientId ? templateSrv.replace(query.clientId, scopedVars) : query.clientId,
      eventType: query.eventType ? templateSrv.replace(query.eventType, scopedVars) : query.eventType,
    };
  }

  filterQuery(query: KeycloakQuery): boolean {
    // A query type must be selected before the query is sent to the backend.
    return !!query.queryType;
  }
}

import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { KeycloakQuery, KeycloakDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, KeycloakQuery, KeycloakDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);

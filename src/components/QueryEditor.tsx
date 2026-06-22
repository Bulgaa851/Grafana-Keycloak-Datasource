import React, { ChangeEvent } from 'react';
import { InlineField, Input, Select, Stack } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { KeycloakDataSourceOptions, KeycloakQuery, KeycloakQueryType } from '../types';

type Props = QueryEditorProps<DataSource, KeycloakQuery, KeycloakDataSourceOptions>;

const QUERY_TYPE_OPTIONS: Array<SelectableValue<KeycloakQueryType>> = [
  { label: 'User count', value: 'user_count', description: 'Total users in the realm' },
  { label: 'Event count (time series)', value: 'event_count', description: 'Realm events over the dashboard time range' },
  { label: 'Active sessions', value: 'active_sessions', description: 'Active sessions for a given client' },
  { label: 'Realm roles', value: 'realm_roles', description: 'All realm roles (table)' },
  { label: 'Groups count', value: 'groups_count', description: 'Total groups in the realm' },
];

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onQueryTypeChange = (selected: SelectableValue<KeycloakQueryType>) => {
    onChange({ ...query, queryType: selected.value ?? 'user_count' });
    onRunQuery();
  };

  const onEventTypeChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, eventType: event.target.value });
  };

  const onClientIdChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange({ ...query, clientId: event.target.value });
  };

  const { queryType, eventType, clientId } = query;

  return (
    <Stack gap={1} direction="column">
      <InlineField label="Query type" labelWidth={18} tooltip="Which Keycloak metric to retrieve.">
        <Select
          inputId="query-editor-query-type"
          options={QUERY_TYPE_OPTIONS}
          value={queryType}
          onChange={onQueryTypeChange}
          width={36}
        />
      </InlineField>

      {queryType === 'event_count' && (
        <InlineField label="Event type" labelWidth={18} tooltip="Optional Keycloak event type filter, e.g. LOGIN. Leave empty for all event types.">
          <Input
            id="query-editor-event-type"
            onChange={onEventTypeChange}
            onBlur={onRunQuery}
            value={eventType ?? ''}
            placeholder="LOGIN (optional)"
            width={36}
          />
        </InlineField>
      )}

      {queryType === 'active_sessions' && (
        <InlineField label="Client ID" labelWidth={18} tooltip="The clientId whose active sessions are counted.">
          <Input
            id="query-editor-client-id"
            onChange={onClientIdChange}
            onBlur={onRunQuery}
            value={clientId ?? ''}
            placeholder="account"
            width={36}
          />
        </InlineField>
      )}
    </Stack>
  );
}

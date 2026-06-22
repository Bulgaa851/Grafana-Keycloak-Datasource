import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, FieldSet } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { KeycloakDataSourceOptions, KeycloakSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<KeycloakDataSourceOptions, KeycloakSecureJsonData> {}

const LABEL_WIDTH = 20;
const INPUT_WIDTH = 50;

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onJsonDataChange = (key: keyof KeycloakDataSourceOptions) => (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: { ...jsonData, [key]: event.target.value },
    });
  };

  // Client secret is sensitive: it is only ever written to secureJsonData.
  const onClientSecretChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: { ...options.secureJsonData, clientSecret: event.target.value },
    });
  };

  const onResetClientSecret = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: { ...options.secureJsonFields, clientSecret: false },
      secureJsonData: { ...options.secureJsonData, clientSecret: '' },
    });
  };

  return (
    <FieldSet label="Keycloak connection">
      <InlineField label="Keycloak base URL" labelWidth={LABEL_WIDTH} interactive tooltip="Base URL of your Keycloak server, without a trailing slash.">
        <Input
          id="config-editor-base-url"
          onChange={onJsonDataChange('baseUrl')}
          value={jsonData.baseUrl ?? ''}
          placeholder="https://keycloak.example.com"
          width={INPUT_WIDTH}
        />
      </InlineField>

      <InlineField label="Realm" labelWidth={LABEL_WIDTH} interactive tooltip="The Keycloak realm to query (e.g. master).">
        <Input
          id="config-editor-realm"
          onChange={onJsonDataChange('realm')}
          value={jsonData.realm ?? ''}
          placeholder="master"
          width={INPUT_WIDTH}
        />
      </InlineField>

      <InlineField label="Client ID" labelWidth={LABEL_WIDTH} interactive tooltip="The confidential client used for the client_credentials grant.">
        <Input
          id="config-editor-client-id"
          onChange={onJsonDataChange('clientId')}
          value={jsonData.clientId ?? ''}
          placeholder="grafana-ds"
          width={INPUT_WIDTH}
        />
      </InlineField>

      <InlineField label="Client Secret" labelWidth={LABEL_WIDTH} interactive tooltip="Stored encrypted in secureJsonData; never returned to the browser.">
        <SecretInput
          required
          id="config-editor-client-secret"
          isConfigured={Boolean(secureJsonFields?.clientSecret)}
          value={secureJsonData?.clientSecret ?? ''}
          placeholder="Enter the client secret"
          width={INPUT_WIDTH}
          onReset={onResetClientSecret}
          onChange={onClientSecretChange}
        />
      </InlineField>
    </FieldSet>
  );
}

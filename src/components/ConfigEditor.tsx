import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onAccountIDChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        accountId: event.target.value,
      },
    });
  };

  const onDatabaseIDChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        databaseId: event.target.value,
      },
    });
  };

  // Secure field (only sent to the backend)
  const onAPITokenChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        apiToken: event.target.value,
      },
    });
  };

  const onResetAPIToken = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        apiToken: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        apiToken: '',
      },
    });
  };

  return (
    <>
      <InlineField label="Account ID" labelWidth={14} interactive tooltip={'Your Cloudflare Account ID'}>
        <Input
          id="config-editor-account-id"
          onChange={onAccountIDChange}
          value={jsonData.accountId || ''}
          placeholder="Enter your Cloudflare Account ID"
          width={40}
        />
      </InlineField>
      <InlineField label="Database ID" labelWidth={14} interactive tooltip={'Your Cloudflare D1 Database ID'}>
        <Input
          id="config-editor-database-id"
          onChange={onDatabaseIDChange}
          value={jsonData.databaseId || ''}
          placeholder="Enter your Cloudflare D1 Database ID"
          width={40}
        />
      </InlineField>
      <InlineField label="API Token" labelWidth={14} interactive tooltip={'Your Cloudflare API Token (scoped to D1)'}>
        <SecretInput
          required
          id="config-editor-api-token"
          isConfigured={secureJsonFields.apiToken}
          value={secureJsonData?.apiToken}
          placeholder="Enter your Cloudflare API Token"
          width={40}
          onReset={onResetAPIToken}
          onChange={onAPITokenChange}
        />
      </InlineField>
    </>
  );
}

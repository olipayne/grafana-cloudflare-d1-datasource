import React, { ChangeEvent } from 'react';
import { InlineField, TextArea } from '@grafana/ui';
import { QueryEditorProps } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onQueryTextChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...query, queryText: event.target.value });
  };

  const { queryText } = query;

  return (
    <InlineField label="SQL Query" labelWidth={10} grow>
      <TextArea
        id="query-editor-query-text"
        onChange={onQueryTextChange}
        value={queryText || ''}
        required
        placeholder="Enter your D1 SQL query (e.g., SELECT * FROM customers)"
        rows={5}
      />
    </InlineField>
  );
}

import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByLabel('SQL Query')).toBeVisible();
});

test('sends the edited SQL to the D1 datasource', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.mockQueryDataResponse({ results: { A: { frames: [] } } });
  await panelEditPage.datasource.set(ds.name);

  const sql = 'SELECT 42 AS answer';
  await panelEditPage.getQueryEditorRow('A').getByLabel('SQL Query').fill(sql);
  const response = await panelEditPage.refreshPanel({
    waitForResponsePredicateCallback: (candidate) =>
      candidate.url().includes('/api/ds/query') &&
      candidate.request().postDataJSON()?.queries?.some((query: { queryText?: string }) => query.queryText === sql),
  });
  expect(response.ok()).toBe(true);
  expect(response.request().postDataJSON().queries).toContainEqual(
    expect.objectContaining({ refId: 'A', queryText: sql, datasource: expect.objectContaining({ type: ds.type }) })
  );
});

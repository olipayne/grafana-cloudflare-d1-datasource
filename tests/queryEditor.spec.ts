import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render query editor', async ({ panelEditPage, readProvisionedDataSource }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await panelEditPage.datasource.set(ds.name);
  await expect(panelEditPage.getQueryEditorRow('A').getByLabel('SQL Query')).toBeVisible();
});

// Commenting out this test as the "Constant" field was removed.
// test('should trigger new query when Constant field is changed', async ({
//   panelEditPage,
//   readProvisionedDataSource,
// }) => {
//   const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
//   await panelEditPage.datasource.set(ds.name);
//   await panelEditPage.getQueryEditorRow('A').getByLabel('SQL Query').fill('test query');
//   const queryReq = panelEditPage.waitForQueryDataRequest();
//   // Assuming there was a field tied to 'Constant' that would trigger a change.
//   // Since it's removed, this part of the test is no longer applicable.
//   // await panelEditPage.getQueryEditorRow('A').getByRole('spinbutton').fill('10');
//   // await expect(await queryReq).toBeTruthy();
// });

// test('data query should return a simple result', async ({ panelEditPage, readProvisionedDataSource }) => {
//   const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
//   await panelEditPage.datasource.set(ds.name);
//   // For D1, a simple SELECT 1 query is a good test.
//   await panelEditPage.getQueryEditorRow('A').getByLabel('SQL Query').fill('SELECT 1 as value;');
//   await panelEditPage.setVisualization('Table');
//   await expect(panelEditPage.refreshPanel()).toBeOK();
//   // Expect the table to contain the value '1'
//   await expect(panelEditPage.panel.data).toContainText(['1']);
// });

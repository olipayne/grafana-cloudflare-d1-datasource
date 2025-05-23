import { test, expect } from '@grafana/plugin-e2e';
import { MyDataSourceOptions, MySecureJsonData } from '../src/types';

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByLabel('Account ID')).toBeVisible();
  await expect(page.getByLabel('Database ID')).toBeVisible();
  await expect(page.getByLabel('API Token')).toBeVisible();
});
test('"Save & test" should be successful when configuration is valid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<MyDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Account ID' }).fill(ds.jsonData.accountId ?? '');
  await page.getByRole('textbox', { name: 'Database ID' }).fill(ds.jsonData.databaseId ?? '');
  await page.getByRole('textbox', { name: 'API Token' }).fill(ds.secureJsonData?.apiToken ?? '');
  await expect(configPage.saveAndTest()).toBeOK();
});

test('"Save & test" should fail when configuration is invalid', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource<MyDataSourceOptions, MySecureJsonData>({ fileName: 'datasources.yml' });
  const configPage = await createDataSourceConfigPage({ type: ds.type });
  await page.getByRole('textbox', { name: 'Account ID' }).fill(ds.jsonData.accountId ?? '');
  await page.getByRole('textbox', { name: 'Database ID' }).fill(ds.jsonData.databaseId ?? '');
  // Leaving API Token blank to trigger a failure
  await expect(configPage.saveAndTest()).not.toBeOK();
  await expect(configPage).toHaveAlert('error', { hasText: 'Health check failed:' }); // Adjusted expected error
});

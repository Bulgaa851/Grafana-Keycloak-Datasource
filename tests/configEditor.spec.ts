import { test, expect } from '@grafana/plugin-e2e';

test('smoke: should render config editor with Keycloak fields', async ({
  createDataSourceConfigPage,
  readProvisionedDataSource,
  page,
}) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByLabel('Keycloak base URL')).toBeVisible();
  await expect(page.getByLabel('Realm')).toBeVisible();
  await expect(page.getByLabel('Client ID')).toBeVisible();
  await expect(page.getByLabel('Client Secret')).toBeVisible();
});

import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

test('showsATreatableErrorInsteadOfHangingOnASlowRoute', async ({ page }) => {
  await entrarComo(page, 'admin-superlogica');
  await page.route('**/api/skills', (rota) => rota.abort('timedout'));
  await page.goto('/studio/skills');

  await expect(page.getByRole('button', { name: 'Tentar de novo' })).toBeVisible({
    timeout: 20_000,
  });
});

test('showsATreatableErrorWhenOffline', async ({ page, context }) => {
  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/skills');
  await page.waitForLoadState('networkidle');

  await context.setOffline(true);
  await page.reload();

  await expect(page.getByRole('button', { name: 'Tentar de novo' })).toBeVisible({
    timeout: 20_000,
  });
  await context.setOffline(false);
});

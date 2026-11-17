import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

// Smoke curto, roda em chromium/firefox/webkit (ver playwright.config.ts): só confirma que o
// Studio carrega e a navegação principal existe, sem exercitar fluxo de negócio.
test('loadsShellWithMainNavigation', async ({ page }) => {
  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/');

  await expect(page.getByText('Studio')).toBeVisible();
  for (const rotulo of ['Concierge', 'Skills', 'Ferramentas', 'Condomínios', 'Avançado']) {
    await expect(page.getByRole('link', { name: rotulo })).toBeVisible();
  }
});

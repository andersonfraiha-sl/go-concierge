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

test('showsATreatableErrorWhenOffline', async ({ page }) => {
  await entrarComo(page, 'admin-superlogica');
  // context.setOffline(true) derrubaria até o documento/JS do próprio 127.0.0.1 (sem servidor
  // de verdade pra simular nesta suíte), não só a API; abortar a rota dos dados da tela
  // reproduz o "sem rede" sem matar a navegação local nem a checagem de usuário (/api/eu).
  await page.route('**/api/skills', (rota) => rota.abort('internetdisconnected'));
  await page.goto('/studio/skills');

  await expect(page.getByRole('button', { name: 'Tentar de novo' })).toBeVisible({
    timeout: 20_000,
  });
});

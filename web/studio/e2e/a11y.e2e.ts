import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

const TELAS: [nome: string, caminho: string][] = [
  ['Concierge', '/studio/'],
  ['Skills', '/studio/skills'],
  ['Ferramentas', '/studio/ferramentas'],
  ['Condomínios', '/studio/condominios'],
  ['Avançado', '/studio/avancado'],
];

for (const [nome, caminho] of TELAS) {
  test(`hasNoSeriousAxeViolationsOn${nome.replace(/\s/g, '')}`, async ({ page }) => {
    await entrarComo(page, 'admin-superlogica');
    await page.goto(caminho);
    await page.waitForLoadState('networkidle');

    const resultado = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa']).analyze();
    const graves = resultado.violations.filter(
      (v) => v.impact === 'serious' || v.impact === 'critical',
    );
    expect(graves, JSON.stringify(graves, null, 2)).toEqual([]);
  });
}

test('keyboardOnlyNavigationHasVisibleFocusAndEscClosesDialogs', async ({ page }) => {
  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/avancado');

  await page.getByRole('button', { name: 'politicas.md' }).first().focus();
  await expect(page.locator(':focus')).toBeVisible();

  // Tab avança pela ordem natural do DOM; o foco visível é o critério, não um alvo específico.
  await page.keyboard.press('Tab');
  await expect(page.locator(':focus')).toBeVisible();

  // "Importar" (na aba Importar e exportar) só habilita depois de escolher destino e zip; "Novo
  // arquivo" abre diálogo sem pré-condição, e já está na aba atual (Arquivos).
  await page.getByRole('button', { name: 'Novo arquivo' }).click();
  await expect(page.getByRole('dialog')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.getByRole('dialog')).not.toBeVisible();
});

test('respectsPrefersReducedMotion', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/');
  await expect(page.getByText('Studio')).toBeVisible();
  // ponytail: só confirma que a tela carrega sob reduced-motion; não há animação própria
  // a verificar hoje (MUI já respeita a media query). Ampliar se o Studio ganhar transições.
});

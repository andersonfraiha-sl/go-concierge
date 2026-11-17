import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

const LARGURAS = [375, 768, 1280, 1920];

for (const largura of LARGURAS) {
  test(`noHorizontalScrollOrCutoffContentAt${largura}px`, async ({ page }) => {
    await page.setViewportSize({ width: largura, height: 900 });
    await entrarComo(page, 'admin-superlogica');
    await page.goto('/studio/skills');
    await page.waitForLoadState('networkidle');

    const [larguraDoDocumento, larguraDaJanela] = await page.evaluate(() => [
      document.documentElement.scrollWidth,
      document.documentElement.clientWidth,
    ]);
    expect(
      larguraDoDocumento,
      `scrollWidth ${larguraDoDocumento} > viewport ${larguraDaJanela}`,
    ).toBeLessThanOrEqual(larguraDaJanela);
  });
}

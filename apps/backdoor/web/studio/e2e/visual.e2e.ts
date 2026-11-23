import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

// Roda só no projeto "desktop" (1440x980, ver playwright.config.ts). Baseline comitada em
// e2e/visual.e2e.ts-snapshots/.
const TELAS: [nome: string, caminho: string][] = [
  ['concierge', '/studio/'],
  ['skills', '/studio/skills'],
  ['condominios', '/studio/condominios'],
];

for (const [nome, caminho] of TELAS) {
  test(`matchesBaselineScreenshotOf${nome}`, async ({ page }) => {
    test.skip(test.info().project.name !== 'desktop', 'baseline é só pra 1440x980');
    await entrarComo(page, 'admin-superlogica');
    await page.goto(caminho);
    await page.waitForLoadState('networkidle');
    await expect(page).toHaveScreenshot(`${nome}.png`, { maxDiffPixelRatio: 0.02 });
  });
}

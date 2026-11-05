import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

// Seed fixa: falha sempre reproduz igual. 60 s de cliques em elementos clicáveis visíveis.
function mulberry32(seed: number): () => number {
  let a = seed;
  return () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

test('sixtySecondsOfSeededRandomClicksProduceNoConsoleErrors', async ({ page }) => {
  const aleatorio = mulberry32(42);
  const erros: string[] = [];
  page.on('console', (msg) => {
    if (msg.type() === 'error') erros.push(msg.text());
  });
  page.on('pageerror', (erro) => erros.push(String(erro)));

  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/');
  await page.waitForLoadState('networkidle');

  const fim = Date.now() + 60_000;
  while (Date.now() < fim) {
    const alvos = await page.locator('button, a, [role="tab"]').all();
    if (alvos.length === 0) break;
    const alvo = alvos[Math.floor(aleatorio() * alvos.length)];
    try {
      await alvo.click({ timeout: 1000, force: true });
    } catch {
      // alvo pode ter saído do DOM entre o sorteio e o clique (navegação); ignora e segue.
    }
    await page.waitForTimeout(150);
  }

  expect(erros, erros.join('\n')).toEqual([]);
});

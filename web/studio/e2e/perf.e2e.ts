import { readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

// Medido com `vite build` em 05/11/2026: maior chunk (índice) ficou em ~652 KB. O orçamento
// abaixo dá folga de ~15% sobre isso; se o build normal passar disso, algo cresceu sem querer.
const ORCAMENTO_DO_MAIOR_CHUNK_BYTES = 750_000;

test('noBuiltChunkExceedsTheMeasuredBudget', () => {
  const pastaDeAssets = join(__dirname, '..', 'dist', 'assets');
  const maiorAtual = readdirSync(pastaDeAssets)
    .filter((arquivo) => arquivo.endsWith('.js'))
    .map((arquivo) => statSync(join(pastaDeAssets, arquivo)).size)
    .reduce((maior, tamanho) => Math.max(maior, tamanho), 0);

  expect(maiorAtual, `maior chunk tem ${maiorAtual} bytes`).toBeLessThanOrEqual(
    ORCAMENTO_DO_MAIOR_CHUNK_BYTES,
  );
});

test('capturesWebVitalsApproximationsOnTheMainScreen', async ({ page }) => {
  await entrarComo(page, 'admin-superlogica');
  await page.goto('/studio/');
  await page.waitForLoadState('networkidle');

  const metricas = await page.evaluate(
    () =>
      new Promise<{ lcp: number; cls: number }>((resolver) => {
        let cls = 0;
        new PerformanceObserver((lista) => {
          for (const entrada of lista.getEntries() as unknown as {
            hadRecentInput: boolean;
            value: number;
          }[]) {
            if (!entrada.hadRecentInput) cls += entrada.value;
          }
        }).observe({ type: 'layout-shift', buffered: true });

        new PerformanceObserver((lista) => {
          const entradas = lista.getEntries();
          const ultima = entradas.at(-1);
          resolver({ lcp: ultima?.startTime ?? 0, cls });
        }).observe({ type: 'largest-contentful-paint', buffered: true });

        // LCP só finaliza no próximo evento de input/visibilitychange; força a leitura.
        setTimeout(() => resolver({ lcp: performance.now(), cls }), 2000);
      }),
  );

  // Orçamento generoso: é dev local servindo mock, não produção; o alvo é pegar regressão
  // grosseira (tela que passa a travar), não medir performance real de rede.
  expect(metricas.lcp, `LCP ${metricas.lcp}ms`).toBeLessThan(4000);
  expect(metricas.cls, `CLS ${metricas.cls}`).toBeLessThan(0.1);
});

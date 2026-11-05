import { fileURLToPath } from 'node:url';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  base: '/studio/',
  plugins: [react()],
  resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
  build: { outDir: 'dist', emptyOutDir: true },
  server: { proxy: { '/api': 'http://127.0.0.1:8090' } },
  // user-event + jsdom passam de 5 s por teste com a suíte inteira em paralelo.
  // e2e/ (Playwright) e stryker.conf.json têm runner e config próprios, fora deste comando.
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    testTimeout: 15000,
    exclude: ['**/node_modules/**', 'e2e/**'],
    // isolate:true (padrão) sobe um processo por arquivo (68 aqui); sob carga do SO (Playwright,
    // Go, Docker rodando junto) um worker atrasado estoura o timeout e o arquivo falha sozinho,
    // sem relação com o teste em si. isolate:false reaproveita os workers entre arquivos: menos
    // processos, menos exposto a contenção de CPU — a causa raiz da instabilidade, não só um
    // timeout maior.
    isolate: false,
  },
});

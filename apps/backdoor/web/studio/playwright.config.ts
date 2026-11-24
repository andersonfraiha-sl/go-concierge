import { defineConfig, devices } from '@playwright/test';

// Servidor Go já compilado e com o Studio buildado (bin/e2e.exe), numa porta própria,
// sem nenhuma integração externa (GRUVI/NEXUS/ATENDE vazios, LLM em mock).
const PORTA = 18091;
const BASE_URL = `http://127.0.0.1:${PORTA}`;

export default defineConfig({
  testDir: './e2e',
  testMatch: /.*\.e2e\.ts/,
  outputDir: './e2e-results',
  timeout: 30_000,
  fullyParallel: false,
  retries: 0,
  reporter: [['list']],
  use: {
    baseURL: BASE_URL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  // Quem sobe/derruba o servidor é o e2e.sh (Postgres próprio + bin/e2e.exe); o Playwright só
  // espera a porta responder, pra não ficar sem controle de quem mata o quê no final.
  webServer: {
    command: 'node -e "setInterval(()=>{},1000)"',
    url: `${BASE_URL}/studio/`,
    reuseExistingServer: true,
    timeout: 5_000,
  },
  projects: [
    {
      name: 'desktop',
      use: { ...devices['Desktop Chrome'], viewport: { width: 1440, height: 980 } },
    },
    // Suíte inteira (não só o smoke) também em WebKit, pro requisito de cross-browser cobrir
    // jornadas e a11y, não só "a tela carrega". visual.e2e.ts pula aqui (baseline é só 1440x980
    // no projeto "desktop").
    {
      name: 'desktop-webkit',
      use: { ...devices['Desktop Safari'], viewport: { width: 1440, height: 980 } },
    },
    { name: 'smoke-chromium', use: { ...devices['Desktop Chrome'] }, testMatch: /smoke\.e2e\.ts/ },
    { name: 'smoke-webkit', use: { ...devices['Desktop Safari'] }, testMatch: /smoke\.e2e\.ts/ },
    { name: 'smoke-firefox', use: { ...devices['Desktop Firefox'] }, testMatch: /smoke\.e2e\.ts/ },
  ],
});

// load.js: carga sustentada no pico esperado do Studio. 20 usuários simultâneos é o
// pico razoável para o Studio de catálogo (edição de skills por um time pequeno, não
// tráfego de público); cada "usuário" navega catálogo → skill → árvore do Avançado e, de
// vez em quando, salva um arquivo (round-trip idempotente: grava o conteúdo que já
// estava lá, para não alterar o catálogo entre rodadas).
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS_GET = { 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };
const HEADERS_PUT = Object.assign({ 'Content-Type': 'application/json' }, HEADERS_GET);

export const options = {
  scenarios: {
    pico_studio: {
      executor: 'constant-vus',
      vus: Number(__ENV.VUS || 20),
      duration: __ENV.DURATION || '2m',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],
    http_req_duration: ['p(95)<800', 'p(99)<2000'],
  },
};

// conteúdo real de rules/output.md, gravado de volta sem alteração (ver GET lido no boot
// do teste, abaixo em setup()).
export function setup() {
  const r = http.get(`${BASE}/api/arquivo?agente=agentes/concierge&caminho=rules/output.md`, { headers: HEADERS_GET });
  if (r.status !== 200) {
    throw new Error(`setup: GET rules/output.md falhou (${r.status}): ${r.body}`);
  }
  return { conteudo: r.json('conteudo') };
}

export default function (data) {
  const catalogo = http.get(`${BASE}/api/skills`, { headers: HEADERS_GET, tags: { nome: 'catalogo' } });
  check(catalogo, { 'catalogo 200': (r) => r.status === 200 });

  const skill = http.get(`${BASE}/api/skills/agendar-mudanca`, { headers: HEADERS_GET, tags: { nome: 'skill' } });
  check(skill, { 'skill 200': (r) => r.status === 200 });

  const arvore = http.get(`${BASE}/api/agentes`, { headers: HEADERS_GET, tags: { nome: 'arvore_avancado' } });
  check(arvore, { 'arvore 200': (r) => r.status === 200 });

  // 1 em cada 5 iterações grava o arquivo (idempotente: mesmo conteúdo de volta).
  if (Math.random() < 0.2) {
    const body = JSON.stringify({ agente: 'agentes/concierge', caminho: 'rules/output.md', conteudo: data.conteudo });
    const put = http.put(`${BASE}/api/arquivo`, body, { headers: HEADERS_PUT, tags: { nome: 'put_arquivo' } });
    check(put, { 'put arquivo 200': (r) => r.status === 200 });
  }

  sleep(1);
}

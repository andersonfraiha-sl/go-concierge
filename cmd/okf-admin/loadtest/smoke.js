// smoke.js: checagem rápida de que o servidor responde nas rotas principais, antes de
// qualquer teste de carga maior. 1 VU, poucas iterações.
import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS = { 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };

export const options = {
  vus: 1,
  iterations: 5,
  thresholds: {
    http_req_failed: ['rate==0'],
  },
};

export default function () {
  const r1 = http.get(`${BASE}/api/skills`, { headers: HEADERS });
  check(r1, { 'skills 200': (r) => r.status === 200 });

  const r2 = http.get(`${BASE}/api/agentes`, { headers: HEADERS });
  check(r2, { 'agentes 200': (r) => r.status === 200 });

  const r3 = http.get(`${BASE}/api/arquivo?agente=agentes/concierge&caminho=agent.yaml`, { headers: HEADERS });
  check(r3, { 'arquivo 200': (r) => r.status === 200 });
}

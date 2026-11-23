// soak.js: carga baixa e constante por um período longo, para expor vazamentos lentos
// (memória, conexões do pool) que uma rajada curta não mostra. Duração via SOAK_DURATION
// (pedido original: 20-30 min; o padrão aqui é mais curto, ver relatório).
import http from 'k6/http';
import { sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS = { 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };

export const options = {
  vus: Number(__ENV.VUS || 5),
  duration: __ENV.SOAK_DURATION || '25m',
  thresholds: {
    http_req_failed: ['rate<0.01'],
  },
};

export default function () {
  http.get(`${BASE}/api/skills`, { headers: HEADERS });
  http.get(`${BASE}/api/eu`, { headers: HEADERS });
  sleep(1);
}

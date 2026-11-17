// stress.js: rampa de VUs até achar o ponto de ruptura (erro > 1% ou p99 > 2s). Só GETs
// (o PUT fica no load.js — aqui queremos saturar leitura do catálogo, que é o ponto fraco
// achado no load.js).
import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS = { 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };

export const options = {
  scenarios: {
    rampa: {
      executor: 'ramping-vus',
      startVUs: 1,
      stages: [
        { duration: '20s', target: 10 },
        { duration: '20s', target: 20 },
        { duration: '20s', target: 40 },
        { duration: '20s', target: 80 },
        { duration: '20s', target: 120 },
      ],
      gracefulRampDown: '5s',
    },
  },
};

export default function () {
  const r = http.get(`${BASE}/api/skills`, { headers: HEADERS });
  check(r, { '200': (resp) => resp.status === 200 });
  sleep(0.2);
}

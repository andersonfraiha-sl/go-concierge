// dependencia_fora.js: carga leve e constante enquanto o Postgres é derrubado e devolvido
// por fora do script (ver relatório). Não falha o teste em erro HTTP: só registra a série
// de status, para o relatório ler o padrão de falha e recuperação.
import http from 'k6/http';
import { sleep } from 'k6';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS = { 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };

export const options = {
  vus: 3,
  duration: __ENV.DURATION || '40s',
};

export default function () {
  const r = http.get(`${BASE}/api/eu`, { headers: HEADERS });
  console.log(`${Date.now()} status=${r.status}`);
  sleep(1);
}

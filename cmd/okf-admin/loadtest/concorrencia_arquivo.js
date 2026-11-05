// concorrencia_arquivo.js: N VUs gravando o MESMO arquivo ao mesmo tempo, cada um com um
// conteúdo diferente e identificável. Depois o teste em Go (ver internal/okf/escrita_test.go
// ou o passo manual no relatório) lê o arquivo final e confere: ele é EXATAMENTE um dos
// conteúdos enviados (último vence, sem mistura/corrupção), nunca uma mistura de dois.
import http from 'k6/http';

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:18092';
const HEADERS = { 'Content-Type': 'application/json', 'X-Dev-Usuario': __ENV.DEV_USUARIO || 'admin-superlogica' };

export const options = {
  vus: Number(__ENV.VUS || 15),
  iterations: Number(__ENV.VUS || 15),
};

// TAMANHO grande o bastante para passar de um único write(2) (testa interleaving real,
// não só a corrida "quem grava por último").
const TAMANHO = Number(__ENV.TAMANHO || 4 * 1024 * 1024);

export default function () {
  const marcador = `vu${__VU}`;
  const marca = marcador.repeat(Math.ceil(TAMANHO / marcador.length)).slice(0, TAMANHO);
  const body = JSON.stringify({ agente: 'agentes/concierge', caminho: 'assets/_teste-concorrencia.md', conteudo: marca });
  http.put(`${BASE}/api/arquivo`, body, { headers: HEADERS });
}

import { vi, type Mock } from 'vitest';

type Rota = (req: Request) => Response | Promise<Response>;

const rotasAusentes: string[] = [];

export function json(corpo: unknown, status = 200): Response {
  return new Response(JSON.stringify(corpo), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

/** Chave "METODO /caminho", sem query. Rota ausente rejeita o fetch e reprova o teste no afterEach. */
export function responderCom(rotas: Record<string, Rota>): Mock<typeof fetch> {
  const falso = vi.fn<typeof fetch>(async (entrada, init) => {
    const req = new Request(new URL(String(entrada), 'http://localhost'), init);
    const chave = `${req.method} ${new URL(req.url).pathname}`;
    const rota = rotas[chave];
    if (!rota) {
      rotasAusentes.push(chave);
      throw new Error(`fetch sem rota falsa: ${chave}`);
    }
    return rota(req);
  });
  vi.stubGlobal('fetch', falso);
  return falso;
}

export function falharSeHouveRotaAusente(): void {
  if (rotasAusentes.length === 0) return;
  const chamadas = rotasAusentes.splice(0).join(', ');
  throw new Error(`fetch sem rota falsa: ${chamadas}`);
}

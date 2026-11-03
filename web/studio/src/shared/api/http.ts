import { z } from 'zod';

export const CHAVE_PAPEL_DEV = 'studio.papelDev';

const TIMEOUT_MS = 15_000;
const MENSAGEM_ERRO_PADRAO = 'Não deu para falar com o servidor.';

type Metodo = 'GET' | 'POST' | 'PUT' | 'DELETE';

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly codigo: string,
    message: string,
    readonly corpo: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// Aceita o formato novo {codigo, mensagem} e o antigo {error}.
// Cada campo é lido por conta própria: um `codigo` malformado não derruba a `mensagem` boa.
const CorpoErro = z.object({
  codigo: z.string().optional().catch(undefined),
  mensagem: z.string().optional().catch(undefined),
  error: z.string().optional().catch(undefined),
});

const CORPO_ERRO_VAZIO: z.infer<typeof CorpoErro> = {};

export function apiGet<S extends z.ZodType>(caminho: string, schema: S): Promise<z.infer<S>> {
  return requisitar('GET', caminho, undefined, schema);
}

export function apiEnviar<S extends z.ZodType>(
  metodo: 'POST' | 'PUT' | 'DELETE',
  caminho: string,
  corpo: unknown,
  schema: S,
): Promise<z.infer<S>> {
  return requisitar(
    metodo,
    caminho,
    corpo === undefined ? undefined : JSON.stringify(corpo),
    schema,
  );
}

/** Corpo binário (ex.: zip): mesmo tratamento de erro, papel e timeout do cliente JSON. */
export function apiEnviarBinario<S extends z.ZodType>(
  metodo: 'POST' | 'PUT',
  caminho: string,
  corpo: BodyInit,
  tipo: string,
  schema: S,
  timeoutMs: number = TIMEOUT_MS,
): Promise<z.infer<S>> {
  return requisitar(metodo, caminho, corpo, schema, { tipo, timeoutMs });
}

type Envio = { tipo: string; timeoutMs: number };
const ENVIO_JSON: Envio = { tipo: 'application/json', timeoutMs: TIMEOUT_MS };

async function requisitar<S extends z.ZodType>(
  metodo: Metodo,
  caminho: string,
  corpo: BodyInit | undefined,
  schema: S,
  envio: Envio = ENVIO_JSON,
): Promise<z.infer<S>> {
  const resposta = await buscar(metodo, caminho, corpo, envio);
  const dados = resposta.status === 204 ? null : await lerJson(resposta);
  if (!resposta.ok) throw erroHttp(resposta.status, dados);
  return interpretar(schema, dados, resposta.status);
}

async function buscar(
  metodo: Metodo,
  caminho: string,
  corpo: BodyInit | undefined,
  envio: Envio,
): Promise<Response> {
  try {
    return await fetch(caminho, {
      method: metodo,
      headers: montarCabecalhos(corpo === undefined ? null : envio.tipo),
      body: corpo ?? null,
      signal: AbortSignal.timeout(envio.timeoutMs),
    });
  } catch (cause) {
    throw new ApiError(0, 'SEM_CONEXAO', 'Sem conexão com o servidor.', cause);
  }
}

function montarCabecalhos(tipoDoCorpo: string | null): Headers {
  const cabecalhos = new Headers({ Accept: 'application/json' });
  if (tipoDoCorpo) cabecalhos.set('Content-Type', tipoDoCorpo);
  const papel = lerPapelDev();
  if (papel) cabecalhos.set('X-Studio-Papel', papel);
  return cabecalhos;
}

function lerPapelDev(): string | null {
  try {
    return localStorage.getItem(CHAVE_PAPEL_DEV);
  } catch (cause) {
    console.warn('ler papel de desenvolvimento:', cause);
    return null;
  }
}

async function lerJson(resposta: Response): Promise<unknown> {
  try {
    return await resposta.json();
  } catch {
    // Corpo vazio ou não-JSON: quem chama decide o erro (HTTP_xxx ou RESPOSTA_INVALIDA).
    return null;
  }
}

function erroHttp(status: number, corpo: unknown): ApiError {
  const lido = CorpoErro.safeParse(corpo);
  const { codigo, mensagem, error } = lido.success ? lido.data : CORPO_ERRO_VAZIO;
  return new ApiError(
    status,
    codigo ?? `HTTP_${status}`,
    mensagem ?? error ?? MENSAGEM_ERRO_PADRAO,
    corpo,
  );
}

function interpretar<S extends z.ZodType>(schema: S, dados: unknown, status: number): z.infer<S> {
  const resultado = schema.safeParse(dados);
  if (resultado.success) return resultado.data;
  console.error('resposta fora do schema:', resultado.error.issues);
  throw new ApiError(
    status,
    'RESPOSTA_INVALIDA',
    'O servidor respondeu num formato inesperado.',
    dados,
  );
}

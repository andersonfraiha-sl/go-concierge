import { z } from 'zod';

export const CHAVE_USUARIO_DEV = 'studio.usuarioDev';

const TIMEOUT_MS = 15_000;
const MENSAGEM_ERRO_PADRAO = 'Não deu para falar com o servidor.';

const MAX_FALHAS = 3;
// 408 e 429 são passageiros mesmo sendo 4xx.
const STATUS_4XX_PASSAGEIROS: ReadonlySet<number> = new Set([408, 429]);

/** Erro do cliente (4xx) não melhora com nova tentativa; rede e 5xx tentam até MAX_FALHAS. */
export function deveTentarDeNovo(falhas: number, erro: unknown): boolean {
  if (erro instanceof ApiError && ehErroDoCliente(erro.status)) return false;
  return falhas < MAX_FALHAS;
}

function ehErroDoCliente(status: number): boolean {
  return status >= 400 && status < 500 && !STATUS_4XX_PASSAGEIROS.has(status);
}

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
  { sinal, timeoutMs = TIMEOUT_MS }: Cancelamento = {},
): Promise<z.infer<S>> {
  const corpoJson = corpo === undefined ? undefined : JSON.stringify(corpo);
  const envio: Envio = { ...ENVIO_JSON, timeoutMs, ...(sinal && { sinal }) };
  return requisitar(metodo, caminho, corpoJson, schema, envio);
}

/** Corpo binário (ex.: zip): mesmo tratamento de erro, usuário de dev e timeout do cliente JSON. */
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

/** `sinal` cancela por quem chama; o prazo `timeoutMs` vale sempre, com ou sem ele. */
export type Cancelamento = { sinal?: AbortSignal; timeoutMs?: number };
type Envio = { tipo: string; timeoutMs: number; sinal?: AbortSignal };
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
    return await fetch(caminho, opcoesDoPedido(metodo, corpo, envio));
  } catch (cause) {
    throw erroDeRede(cause);
  }
}

function opcoesDoPedido(metodo: Metodo, corpo: BodyInit | undefined, envio: Envio): RequestInit {
  return {
    method: metodo,
    headers: montarCabecalhos(corpo === undefined ? null : envio.tipo),
    body: corpo ?? null,
    signal: sinalDe(envio),
  };
}

function sinalDe({ sinal, timeoutMs }: Envio): AbortSignal {
  const prazo = AbortSignal.timeout(timeoutMs);
  return sinal ? AbortSignal.any([sinal, prazo]) : prazo;
}

// fetch rejeita com o motivo do sinal: prazo estourado e cancelamento não são falta de conexão.
const ERROS_DE_SINAL: Record<string, [codigo: string, mensagem: string]> = {
  TimeoutError: ['TEMPO_ESGOTADO', 'O servidor demorou demais para responder.'],
  AbortError: ['CANCELADO', 'O pedido foi cancelado.'],
};

function erroDeRede(cause: unknown): ApiError {
  const nome = typeof cause === 'object' && cause !== null && 'name' in cause ? cause.name : '';
  const [codigo, mensagem] = ERROS_DE_SINAL[String(nome)] ?? [
    'SEM_CONEXAO',
    'Sem conexão com o servidor.',
  ];
  return new ApiError(0, codigo, mensagem, cause);
}

function montarCabecalhos(tipoDoCorpo: string | null): Headers {
  const cabecalhos = new Headers({ Accept: 'application/json' });
  if (tipoDoCorpo) cabecalhos.set('Content-Type', tipoDoCorpo);
  const usuario = lerUsuarioDev();
  if (usuario) cabecalhos.set('X-Dev-Usuario', usuario);
  return cabecalhos;
}

export function lerUsuarioDev(): string | null {
  try {
    return localStorage.getItem(CHAVE_USUARIO_DEV);
  } catch (cause) {
    console.warn('ler usuário de desenvolvimento:', cause);
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

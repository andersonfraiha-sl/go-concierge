export type ContextoDoErro = {
  origem: 'react' | 'janela' | 'promessa';
  rota: string;
  usuarioId: string | null;
  dev: boolean;
  componentStack?: string | undefined;
};

// Mesmos formatos do back: 000.000.000-00 (pontuação opcional) e telefone com DDD.
const CPF = /\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b/g;
const TELEFONE = /(?<!\d)(?:\+?55\s?)?\(?\d{2}\)?\s?9?\d{4}-?\d{4}(?!\d)/g;

export function mascararDocumentos(texto: string): string {
  return texto.replace(CPF, '***.***.***-**').replace(TELEFONE, '(**) *****-****');
}

function comoErro(causa: unknown): Error {
  return causa instanceof Error ? causa : new Error(String(causa));
}

/** Único ponto de saída de erros de tela; trocar o console por um tracker é mexer só aqui. */
export function reportarErro(causa: unknown, contexto: ContextoDoErro): void {
  const erro = comoErro(causa);
  console.error('erro de tela', {
    nome: erro.name,
    mensagem: mascararDocumentos(erro.message),
    stack: erro.stack === undefined ? undefined : mascararDocumentos(erro.stack),
    ...contexto,
  });
}

const CHUNK_FALHOU =
  /dynamically imported module|Importing a module script failed|ChunkLoadError|Loading chunk/i;

export function falhaDeChunk(causa: unknown): boolean {
  return CHUNK_FALHOU.test(comoErro(causa).message);
}

type LerContexto = () => Pick<ContextoDoErro, 'rota' | 'usuarioId' | 'dev'>;

/** Erros fora do React (handlers, timers, promessas soltas). Devolve o desinstalador. */
export function instalarCapturaGlobal(lerContexto: LerContexto): () => void {
  const aoErrar = (e: ErrorEvent) =>
    reportarErro(e.error ?? e.message, { origem: 'janela', ...lerContexto() });
  const aoRejeitar = (e: PromiseRejectionEvent) =>
    reportarErro(e.reason, { origem: 'promessa', ...lerContexto() });
  window.addEventListener('error', aoErrar);
  window.addEventListener('unhandledrejection', aoRejeitar);
  return () => {
    window.removeEventListener('error', aoErrar);
    window.removeEventListener('unhandledrejection', aoRejeitar);
  };
}

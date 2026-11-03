export const mensagemDe = (erro: unknown): string =>
  erro instanceof Error ? erro.message : String(erro);

const CHAVE_ABERTO = 'studio.copilotoAberto';

/** Painel aberto ou fechado fica no navegador; a conversa, não. */
export function lerAberto(): boolean {
  try {
    return localStorage.getItem(CHAVE_ABERTO) === 'true';
  } catch (cause) {
    console.warn('ler preferência do Copiloto:', cause);
    return false;
  }
}

export function gravarAberto(aberto: boolean): void {
  try {
    localStorage.setItem(CHAVE_ABERTO, String(aberto));
  } catch (cause) {
    console.warn('gravar preferência do Copiloto:', cause);
  }
}

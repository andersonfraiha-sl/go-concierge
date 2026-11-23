import { useSyncExternalStore } from 'react';
import { CHAVE_USUARIO_DEV, lerUsuarioDev } from '@/shared/api/http';

// Avisa a própria aba: o evento `storage` do navegador só chega às outras.
const EVENTO_TROCA = 'studio:usuario-dev';

/** Usuário do "Ver como"; muda no mesmo render do clique, sem esperar o cache de queries. */
export function useUsuarioDev(): string | null {
  return useSyncExternalStore(assinar, lerUsuarioDev);
}

export function gravarUsuarioDev(usuario: string): void {
  try {
    localStorage.setItem(CHAVE_USUARIO_DEV, usuario);
  } catch (cause) {
    console.warn('gravar usuário de desenvolvimento:', cause);
  }
  window.dispatchEvent(new Event(EVENTO_TROCA));
}

function assinar(aoMudar: () => void): () => void {
  window.addEventListener(EVENTO_TROCA, aoMudar);
  return () => window.removeEventListener(EVENTO_TROCA, aoMudar);
}

import { useEffect } from 'react';
import { useCopiloto } from './CopilotoProvider';
import type { ContextoDoCopiloto } from './contratos';

/**
 * Diz ao Copiloto o que a tela mostra. `null` quando uma tela de dentro fala por ela (ex.: a aba
 * de fluxo no editor da skill): os efeitos do filho rodam antes dos do pai e seriam sobrescritos.
 */
export function useContextoDaTela(contexto: ContextoDoCopiloto | null): void {
  const { definirContexto } = useCopiloto();
  // Sem lista de dependências de propósito: `definirContexto` ignora o mesmo contexto, então o
  // efeito só muda algo quando o item aberto muda, e nunca guarda uma tela antiga.
  useEffect(() => {
    if (contexto) definirContexto(contexto);
  });
}

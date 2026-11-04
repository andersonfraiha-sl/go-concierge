import { useEffect } from 'react';
import { CONTEXTO_NEUTRO, useCopiloto } from './CopilotoProvider';
import type { ContextoDoCopiloto } from './contratos';

/**
 * Diz ao Copiloto o que a tela mostra; `null` limpa (a tela não tem nada a dizer, ex.: Avançado
 * sem permissão). Uma tela de dentro e a de fora que estejam montadas juntas dizem o mesmo
 * contexto: os efeitos do filho rodam antes dos do pai, e o pai não pode desdizer o filho.
 */
export function useContextoDaTela(contexto: ContextoDoCopiloto | null): void {
  const { definirContexto } = useCopiloto();
  // Sem lista de dependências de propósito: `definirContexto` ignora o mesmo contexto, então o
  // efeito só muda algo quando o que a tela mostra muda.
  useEffect(() => {
    definirContexto(contexto ?? CONTEXTO_NEUTRO);
  });
}

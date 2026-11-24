import { useState } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { useAutosave } from '@/shared/autosave/useAutosave';
import {
  type Ajustes,
  type Estado,
  PADRAO,
  salvarAjustes,
  useAplicarMapeamento,
  useEscopo,
  type Valor,
} from './contratos';

type Entrada = { estado: Estado; condominio: string; ferramentas: Ferramenta[] };

/**
 * Ajustes do condomínio com autosave. Fora do react-hook-form: Valor é JSON recursivo e os
 * tipos de caminho do RHF não instanciam sobre ele (TS2589). Cada CampoAjuste já é um campo
 * controlado deste mapa.
 */
export type AjustesEmEdicao = ReturnType<typeof useAjustes>;

export function useAjustes({ estado, condominio, ferramentas }: Entrada) {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const gravados = estado.mapeamento.payloads;

  const [ajustes, setAjustes] = useState<Ajustes>(() =>
    Object.fromEntries(ferramentas.map((f) => [f.name, gravados[condominio]?.[f.name] ?? {}])),
  );

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: ajustes,
    salvar: async (valor) => {
      // Mescla com o que está no cache agora: outras seções gravam a própria parte deste mesmo
      // mapa, e esta não pode apagá-la.
      const atuais = gravados[condominio] ?? {};
      aplicar(await salvarAjustes({ condominio, ajustes: { ...atuais, ...valor } }, escopo));
    },
  });

  const mudar = (recurso: string, campos: Record<string, Valor>): void =>
    setAjustes((atual) => ({ ...atual, [recurso]: { ...atual[recurso], ...campos } }));

  const herdadoDe = (recurso: string): Record<string, Valor> =>
    condominio === PADRAO ? {} : (gravados[PADRAO]?.[recurso] ?? {});

  return { valores: ajustes, mudar, herdadoDe, salvamento, tentarDeNovo };
}

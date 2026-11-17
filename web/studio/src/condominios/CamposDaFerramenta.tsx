import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoAjuste } from './CampoAjuste';
import type { AjustesEmEdicao } from './useAjustes';

type Props = { ferramenta: Ferramenta; ajustes: AjustesEmEdicao; editavel: boolean };

/** Um CampoAjuste por definição da ferramenta, com o valor do condomínio e o que ele herda. */
export function CamposDaFerramenta({ ferramenta, ajustes, editavel }: Props): ReactNode {
  const valores = ajustes.valores[ferramenta.name];
  const herdados = ajustes.herdadoDe(ferramenta.name);

  return ferramenta.definicoes.map((definicao) => (
    <CampoAjuste
      key={definicao.name}
      definicao={definicao}
      valor={valores?.[definicao.name]}
      herdado={herdados[definicao.name] ?? definicao.default}
      desabilitado={!editavel}
      onMudar={(valor) => ajustes.mudar(ferramenta.name, { [definicao.name]: valor })}
    />
  ));
}

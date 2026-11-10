import { type Dispatch, type ReactNode, type SetStateAction, useMemo } from 'react';
import type { Fluxo, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import type { CatalogoDeFerramentas } from '@/fluxo/painel/camposPorTipo';
import type { EstadoSalvamento } from '@/shared/autosave/useAutosave';
import { BarraDoFluxo } from '@/skills/abas/BarraDoFluxo';
import { AreaDoFluxo, CanvasEPainel, useEdicaoDosBlocos } from '@/skills/abas/FluxoAba';
import { useCatalogoDeFerramentas } from '@/skills/abas/fluxoDaSkill';

type Props = {
  skill: string;
  fluxo: Fluxo;
  onMudarFluxo: Dispatch<SetStateAction<Fluxo>>;
  editavel: boolean;
  /** Ferramentas que a base usa: a camada não liga nenhuma fora delas. */
  ferramentasDaBase: readonly string[];
  /** Locais e do servidor, já juntos: quem salva também precisa deles para bloquear o autosave. */
  problemas: ProblemaFluxo[];
  salvamento: EstadoSalvamento;
  onTentarDeNovo: () => void;
};

/** O canvas da base, sem a aba Ferramentas e só com as ferramentas da base. */
export function FluxoDaCamada(props: Props): ReactNode {
  const { skill, fluxo, onMudarFluxo, editavel, ferramentasDaBase, problemas } = props;
  const catalogo = useCatalogoDaBase(ferramentasDaBase);
  const edicao = useEdicaoDosBlocos(fluxo, onMudarFluxo);

  return (
    <AreaDoFluxo>
      <BarraDoFluxo
        editavel={editavel}
        totalDePassos={fluxo.blocos.length}
        problemas={problemas}
        doFluxoTodo={problemas.filter((p) => p.blocoId === undefined)}
        salvamento={props.salvamento}
        onTentarDeNovo={props.onTentarDeNovo}
      />
      <CanvasEPainel
        nome={skill}
        fluxo={fluxo}
        problemas={problemas}
        catalogo={catalogo}
        editavel={editavel}
        edicao={edicao}
        onMudarFluxo={onMudarFluxo}
      />
    </AreaDoFluxo>
  );
}

// Filtrar o catálogo basta para travar: o seletor só oferece o que sobra, e uma ferramenta
// de fora aparece como "não encontrada" no bloco.
function useCatalogoDaBase(ferramentasDaBase: readonly string[]): CatalogoDeFerramentas {
  const catalogo = useCatalogoDeFerramentas();
  return useMemo(() => {
    if (catalogo.estado !== 'pronto') return catalogo;
    const daBase = new Set(ferramentasDaBase);
    return {
      estado: 'pronto',
      ferramentas: catalogo.ferramentas.filter((ferramenta) => daBase.has(ferramenta.name)),
    };
  }, [catalogo, ferramentasDaBase]);
}

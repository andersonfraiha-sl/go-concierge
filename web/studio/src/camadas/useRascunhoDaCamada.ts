import { useQueryClient } from '@tanstack/react-query';
import { useMemo, useRef, useState } from 'react';
import { type Control, useForm } from 'react-hook-form';
import type { Fluxo, ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { validarFluxo } from '@/fluxo/modelo/validar';
import { type EstadoSalvamento, useAutosave } from '@/shared/autosave/useAutosave';
import { useValoresDoFormulario } from '@/shared/autosave/useValoresDoFormulario';
import { juntarProblemas, problemasDaRecusa } from '@/skills/abas/fluxoDaSkill';
import type { Textos } from '@/skills/contratos';
import { semVazios } from '@/skills/textosDoEditor';
import {
  type Camada,
  type CamadaLida,
  CHAVE_DAS_ATUALIZACOES,
  chaveDaCamada,
  chaveDoHistorico,
  type Documento,
  publicarCamada,
  salvarRascunho,
} from './contratos';

type Orientacao = { orientacao: string };

export type RascunhoDaCamada = {
  controlDosTextos: Control<Textos>;
  controlDaOrientacao: Control<Orientacao>;
  fluxo: Fluxo;
  setFluxo: (mudar: Fluxo | ((atual: Fluxo) => Fluxo)) => void;
  problemas: ProblemaFluxo[];
  salvamento: EstadoSalvamento;
  tentarDeNovo: () => Promise<void>;
  publicar: (resumo: string) => Promise<void>;
};

/**
 * O documento da camada em edição: textos e orientação em formulário, fluxo em estado, um só
 * autosave. O servidor guarda o rascunho; a versão que ele devolveu é a que o próximo PUT cita.
 */
export function useRascunhoDaCamada(camada: Camada, lida: CamadaLida): RascunhoDaCamada {
  const cliente = useQueryClient();
  const formDosTextos = useForm<Textos>({ defaultValues: lida.documento.textos });
  const formDaOrientacao = useForm<Orientacao>({
    defaultValues: { orientacao: lida.documento.orientacao },
  });
  const textos = useValoresDoFormulario(formDosTextos.control);
  const { orientacao } = useValoresDoFormulario(formDaOrientacao.control);
  const [fluxo, setFluxo] = useState(lida.documento.fluxo);
  // As ferramentas são da base e a camada nunca as muda: ficam como vieram.
  const [ferramentas] = useState(lida.documento.ferramentas);
  const [doServidor, setDoServidor] = useState(lida.problemas);
  const versao = useRef(lida.versao);

  const documento = useMemo<Documento>(
    () => ({ textos, fluxo, ferramentas, orientacao }),
    [textos, fluxo, ferramentas, orientacao],
  );
  const locais = useMemo(() => validarFluxo(fluxo), [fluxo]);
  const problemas = useMemo(
    () => juntarProblemas(locais, doServidor, fluxo.blocos),
    [locais, doServidor, fluxo.blocos],
  );

  const salvar = async (valor: Documento): Promise<void> => {
    const enviado = { ...valor, textos: semVazios(valor.textos) };
    try {
      const salvo = await salvarRascunho(camada, { versao: versao.current, documento: enviado });
      versao.current = salvo.versao;
      setDoServidor(salvo.problemas);
      // Ao voltar para a tela, o cache precisa ter o rascunho e a versão que acabaram de ser gravados.
      cliente.setQueryData<CamadaLida>(
        chaveDaCamada(camada),
        (atual) =>
          atual && {
            ...atual,
            versao: salvo.versao,
            documento: enviado,
            temRascunho: true,
            problemas: salvo.problemas,
          },
      );
    } catch (erro) {
      const recusados = problemasDaRecusa(erro);
      if (recusados !== null) setDoServidor(recusados);
      throw erro;
    }
  };

  const { estado, tentarDeNovo, salvarAgora } = useAutosave({
    valor: documento,
    salvar,
    bloqueio: locais.length > 0 ? 'corrija o bloco marcado' : null,
  });

  const publicar = async (resumo: string): Promise<void> => {
    // O que ainda espera o atraso do autosave entra na versão publicada.
    await salvarAgora();
    try {
      await publicarCamada(camada, { resumo });
    } catch (erro) {
      const recusados = problemasDaRecusa(erro);
      if (recusados !== null) setDoServidor(recusados);
      throw erro;
    }
    await Promise.all(
      [chaveDaCamada(camada), chaveDoHistorico(camada), CHAVE_DAS_ATUALIZACOES].map((queryKey) =>
        cliente.invalidateQueries({ queryKey }),
      ),
    );
    const fresca = cliente.getQueryData<CamadaLida>(chaveDaCamada(camada));
    if (fresca) versao.current = fresca.versao;
  };

  return {
    controlDosTextos: formDosTextos.control,
    controlDaOrientacao: formDaOrientacao.control,
    fluxo,
    setFluxo,
    problemas,
    salvamento: estado,
    tentarDeNovo,
    publicar,
  };
}

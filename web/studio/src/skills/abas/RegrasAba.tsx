import type { ReactNode } from 'react';
import { ArquivosDaSkillSecao } from './ArquivosDaSkillSecao';
import { regrasDe, tituloDaRegra } from './arquivosDaSkill';
import { RegraArquivo } from './RegraArquivo';

type Props = { nome?: string; caminho: string; titulo?: string };

export function RegrasAba({ caminho, titulo = 'Regras' }: Props): ReactNode {
  return (
    <ArquivosDaSkillSecao
      caminho={caminho}
      titulo={titulo}
      ajuda="Orientações que a skill sempre segue ao responder."
      rotuloNovo="Nova regra"
      mensagemDeErro="Não foi possível carregar as regras."
      fraseVazia="Esta skill ainda não tem regras."
      tituloDaExclusao="Excluir regra"
      textoDaExclusao={(c) =>
        `Excluir a regra "${c === '' ? '' : tituloDaRegra(c)}"? A skill deixa de segui-la.`
      }
      tipo="regra"
      selecionar={(agente) => ({ proprios: regrasDe(agente), herdados: [] })}
      renderArquivo={(regra, ctx) => (
        <RegraArquivo
          key={regra.caminho}
          agente={ctx.caminhoDoAgente}
          caminho={regra.caminho}
          titulo={tituloDaRegra(regra.caminho)}
          editavel={ctx.editavel}
          onExcluir={ctx.onExcluir}
          descartar={ctx.descartar}
        />
      )}
    />
  );
}

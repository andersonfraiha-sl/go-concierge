import type { ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { NomeDialog, type NomeEscolhido } from '@/shared/ui/NomeDialog';
import {
  preencherTextosNovos,
  type SkillResumo,
  useCriarSkill,
  useDuplicarSkill,
  useExcluirSkill,
} from './contratos';
import { NovaSkillDialog } from './NovaSkillDialog';

export type Dialogo =
  | { tipo: 'nova' }
  | { tipo: 'duplicar'; origem: SkillResumo }
  | { tipo: 'excluir'; skill: SkillResumo };

export const caminhoDoEditor = (nome: string): string => `/skills/${encodeURIComponent(nome)}`;

type Props = { dialogo: Dialogo | null; fechar: () => void };

/** Diálogos de criar, duplicar e excluir skill, com as ações que cada um dispara. */
export function DialogosDeSkills({ dialogo, fechar }: Props): ReactNode {
  const navegar = useNavigate();
  const { mostrar } = useSnackbar();
  const criar = useCriarSkill();
  const duplicar = useDuplicarSkill();
  const excluir = useExcluirSkill();

  const criarSkill = async (nova: { nome: string; titulo: string }) => {
    const criada = await criar.mutateAsync(nova);
    mostrar('Skill criada.');
    return criada;
  };

  const abrirEditor = (nome: string) => void navegar(caminhoDoEditor(nome));

  /**
   * Textos não salvaram: a pessoa segue para o editor com o que digitou, levado como estado da
   * navegação (não no cache), para o editor aplicar como alteração pendente e mandar ao servidor.
   */
  const abrirEditorComTextosPendentes = (
    nome: string,
    textos: { resumo: string; quandoUsar: string },
  ) => void navegar(caminhoDoEditor(nome), { state: { textosPendentes: textos } });

  const duplicarEAbrir = async (origem: SkillResumo, nova: NomeEscolhido) => {
    const copia = await duplicar.mutateAsync({ origem: origem.nome, ...nova });
    mostrar('Skill duplicada.');
    void navegar(caminhoDoEditor(copia.nome));
  };

  const excluirSkill = async (skill: SkillResumo) => {
    fechar();
    try {
      await excluir.mutateAsync(skill.nome);
      mostrar('Skill excluída.');
    } catch (erro) {
      mostrar(erro instanceof Error ? erro.message : String(erro), 'error');
    }
  };

  return (
    <>
      {dialogo?.tipo === 'nova' && (
        <NovaSkillDialog
          onCriar={criarSkill}
          onSalvarTextos={preencherTextosNovos}
          onConcluida={abrirEditor}
          onFecharComFalha={abrirEditorComTextosPendentes}
          onCancelar={fechar}
        />
      )}
      {dialogo?.tipo === 'duplicar' && (
        <NomeDialog
          titulo="Duplicar skill"
          tituloInicial={`Cópia de ${dialogo.origem.titulo}`}
          confirmar="Duplicar"
          onEnviar={(nova) => duplicarEAbrir(dialogo.origem, nova)}
          onCancelar={fechar}
        />
      )}
      {dialogo?.tipo === 'excluir' && (
        <ConfirmDialog
          aberto
          titulo="Excluir skill"
          texto={`Excluir "${dialogo.skill.titulo}"? O fluxo e as regras somem junto.`}
          confirmar="Excluir"
          onConfirmar={() => void excluirSkill(dialogo.skill)}
          onCancelar={fechar}
        />
      )}
    </>
  );
}

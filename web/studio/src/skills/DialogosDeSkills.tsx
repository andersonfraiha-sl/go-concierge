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
  const acoes = useAcoesDosDialogos(fechar);
  if (dialogo === null) return null;
  switch (dialogo.tipo) {
    case 'nova':
      return <DialogoNova acoes={acoes} fechar={fechar} />;
    case 'duplicar':
      return <DialogoDuplicar origem={dialogo.origem} acoes={acoes} fechar={fechar} />;
    case 'excluir':
      return <DialogoExcluir skill={dialogo.skill} acoes={acoes} fechar={fechar} />;
  }
}

type Acoes = ReturnType<typeof useAcoesDosDialogos>;

function useAcoesDosDialogos(fechar: () => void) {
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

  return { criarSkill, abrirEditor, abrirEditorComTextosPendentes, duplicarEAbrir, excluirSkill };
}

function DialogoNova({ acoes, fechar }: { acoes: Acoes; fechar: () => void }): ReactNode {
  return (
    <NovaSkillDialog
      onCriar={acoes.criarSkill}
      onSalvarTextos={preencherTextosNovos}
      onConcluida={acoes.abrirEditor}
      onFecharComFalha={acoes.abrirEditorComTextosPendentes}
      onCancelar={fechar}
    />
  );
}

type PropsDoDuplicar = { origem: SkillResumo; acoes: Acoes; fechar: () => void };

function DialogoDuplicar({ origem, acoes, fechar }: PropsDoDuplicar): ReactNode {
  return (
    <NomeDialog
      titulo="Duplicar skill"
      tituloInicial={`Cópia de ${origem.titulo}`}
      confirmar="Duplicar"
      onEnviar={(nova) => acoes.duplicarEAbrir(origem, nova)}
      onCancelar={fechar}
    />
  );
}

type PropsDoExcluir = { skill: SkillResumo; acoes: Acoes; fechar: () => void };

function DialogoExcluir({ skill, acoes, fechar }: PropsDoExcluir): ReactNode {
  return (
    <ConfirmDialog
      aberto
      titulo="Excluir skill"
      texto={`Excluir "${skill.titulo}"? O fluxo e as regras somem junto.`}
      confirmar="Excluir"
      onConfirmar={() => void acoes.excluirSkill(skill)}
      onCancelar={fechar}
    />
  );
}

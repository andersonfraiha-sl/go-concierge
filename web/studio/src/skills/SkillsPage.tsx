import AddIcon from '@mui/icons-material/Add';
import { Box, Button, CircularProgress, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useNavigate } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog, type NomeEscolhido } from '@/shared/ui/NomeDialog';
import {
  type SkillResumo,
  useCriarSkill,
  useDuplicarSkill,
  useExcluirSkill,
  useSkills,
} from './contratos';
import { SkillCard } from './SkillCard';

const FRASE_VAZIA =
  'Nenhuma skill ainda. Uma skill é algo que a assistente sabe fazer pelo morador, como reservar uma área comum.';

type Dialogo =
  | { tipo: 'nova' }
  | { tipo: 'duplicar'; origem: SkillResumo }
  | { tipo: 'excluir'; skill: SkillResumo };

const caminhoDoEditor = (nome: string): string => `/skills/${encodeURIComponent(nome)}`;

export function SkillsPage(): ReactNode {
  const skills = useSkills();
  const navegar = useNavigate();
  const { mostrar } = useSnackbar();
  const criar = useCriarSkill();
  const duplicar = useDuplicarSkill();
  const excluir = useExcluirSkill();
  const [dialogo, setDialogo] = useState<Dialogo | null>(null);

  const podeCriar = usePodeEditar('skills.criar');
  const podeExcluir = usePodeEditar('skills.excluir');
  const fechar = () => setDialogo(null);

  const criarEAbrir = async (nova: NomeEscolhido) => {
    const criada = await criar.mutateAsync(nova);
    mostrar('Skill criada.');
    void navegar(caminhoDoEditor(criada.nome));
  };

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
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <Typography variant="h5" component="h1" sx={{ flex: 1 }}>
          Skills
        </Typography>
        {podeCriar && (
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => setDialogo({ tipo: 'nova' })}
          >
            Nova skill
          </Button>
        )}
      </Box>

      {skills.isPending && (
        <CircularProgress aria-label="Carregando skills" sx={{ alignSelf: 'center' }} />
      )}
      {skills.isError && (
        <ErroComRetry
          mensagem={skills.error.message}
          onTentarDeNovo={() => void skills.refetch()}
        />
      )}
      {skills.isSuccess && skills.data.skills.length === 0 && <EmptyState frase={FRASE_VAZIA} />}
      {skills.isSuccess && skills.data.skills.length > 0 && (
        <Box
          sx={{
            display: 'grid',
            gap: 2,
            gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
          }}
        >
          {skills.data.skills.map((skill) => (
            <SkillCard
              key={skill.nome}
              skill={skill}
              onAbrir={() => void navegar(caminhoDoEditor(skill.nome))}
              {...(podeCriar && {
                onDuplicar: () => setDialogo({ tipo: 'duplicar', origem: skill }),
              })}
              {...(podeExcluir && { onExcluir: () => setDialogo({ tipo: 'excluir', skill }) })}
            />
          ))}
        </Box>
      )}

      {dialogo?.tipo === 'nova' && (
        <NomeDialog
          titulo="Nova skill"
          tituloInicial=""
          confirmar="Criar"
          onEnviar={criarEAbrir}
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
    </Box>
  );
}

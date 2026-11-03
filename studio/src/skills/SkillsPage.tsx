import AddIcon from '@mui/icons-material/Add';
import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { podeNoCatalogo, useEu } from '@/acesso/eu';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { useCriarSkill, useDuplicarSkill, useExcluirSkill, useSkills, type SkillResumo } from './contratos';
import { NovaSkillDialog, type NovaSkill } from './NovaSkillDialog';
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
  const { data: eu } = useEu();
  const navegar = useNavigate();
  const { mostrar } = useSnackbar();
  const criar = useCriarSkill();
  const duplicar = useDuplicarSkill();
  const excluir = useExcluirSkill();
  const [dialogo, setDialogo] = useState<Dialogo | null>(null);

  const podeCriar = eu !== undefined && podeNoCatalogo(eu, 'skills.criar');
  const podeExcluir = eu !== undefined && podeNoCatalogo(eu, 'skills.excluir');
  const fechar = () => setDialogo(null);

  const criarEAbrir = async (nova: NovaSkill) => {
    const criada = await criar.mutateAsync(nova);
    mostrar('Skill criada.');
    void navegar(caminhoDoEditor(criada.nome));
  };

  const duplicarEAbrir = async (origem: SkillResumo, nova: NovaSkill) => {
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
          <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialogo({ tipo: 'nova' })}>
            Nova skill
          </Button>
        )}
      </Box>

      {skills.isPending && <CircularProgress aria-label="Carregando skills" sx={{ alignSelf: 'center' }} />}
      {skills.isError && (
        <Alert severity="error" action={<Button onClick={() => void skills.refetch()}>Tentar de novo</Button>}>
          {skills.error.message}
        </Alert>
      )}
      {skills.isSuccess && skills.data.skills.length === 0 && <EmptyState frase={FRASE_VAZIA} />}
      {skills.isSuccess && skills.data.skills.length > 0 && (
        <Box
          sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))' }}
        >
          {skills.data.skills.map((skill) => (
            <SkillCard
              key={skill.nome}
              skill={skill}
              onAbrir={() => void navegar(caminhoDoEditor(skill.nome))}
              {...(podeCriar && { onDuplicar: () => setDialogo({ tipo: 'duplicar', origem: skill }) })}
              {...(podeExcluir && { onExcluir: () => setDialogo({ tipo: 'excluir', skill }) })}
            />
          ))}
        </Box>
      )}

      {dialogo?.tipo === 'nova' && (
        <NovaSkillDialog
          titulo="Nova skill"
          tituloInicial=""
          confirmar="Criar"
          onEnviar={criarEAbrir}
          onCancelar={fechar}
        />
      )}
      {dialogo?.tipo === 'duplicar' && (
        <NovaSkillDialog
          titulo="Duplicar skill"
          tituloInicial={`Cópia de ${dialogo.origem.titulo}`}
          confirmar="Duplicar"
          onEnviar={(nova) => duplicarEAbrir(dialogo.origem, nova)}
          onCancelar={fechar}
        />
      )}
      <ConfirmDialog
        aberto={dialogo?.tipo === 'excluir'}
        titulo="Excluir skill"
        texto={`Excluir "${dialogo?.tipo === 'excluir' ? dialogo.skill.titulo : ''}"? O fluxo e as regras somem junto.`}
        confirmar="Excluir"
        onConfirmar={() => dialogo?.tipo === 'excluir' && void excluirSkill(dialogo.skill)}
        onCancelar={fechar}
      />
    </Box>
  );
}

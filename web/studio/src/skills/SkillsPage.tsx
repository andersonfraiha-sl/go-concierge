import AddIcon from '@mui/icons-material/Add';
import { Box, Button, CircularProgress, TextField, Typography } from '@mui/material';
import { type ReactNode, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { useArvore } from '@/avancado/contratos';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { type SkillResumo, useSkills } from './contratos';
import { caminhoDoEditor, type Dialogo, DialogosDeSkills } from './DialogosDeSkills';
import { filtrarPorBusca, type Problemas, problemasPorCaminho } from './estatisticas';
import { GrupoDeSkills } from './GrupoDeSkills';
import { NumerosDasSkills } from './NumerosDasSkills';
import { SkillCard } from './SkillCard';

const FRASE_VAZIA =
  'Uma skill é algo que o concierge sabe fazer pelo morador, como reservar o salão ou mandar a 2ª via do boleto.';

const SEM_PROBLEMAS: ReadonlyMap<string, Problemas> = new Map();

export function SkillsPage(): ReactNode {
  const skills = useSkills();
  const navegar = useNavigate();
  const [dialogo, setDialogo] = useState<Dialogo | null>(null);
  const [busca, setBusca] = useState('');

  const podeCriar = usePodeEditar('skills.criar');
  const podeExcluir = usePodeEditar('skills.excluir');
  const podeVerProblemas = usePodeEditar('skills.editar_fluxo');
  const arvore = useArvore({ enabled: podeVerProblemas });
  const fechar = () => setDialogo(null);

  const todasSkills = skills.data?.skills ?? [];
  const problemas = useMemo(
    () => (podeVerProblemas ? problemasPorCaminho(arvore.data ?? []) : SEM_PROBLEMAS),
    [podeVerProblemas, arvore.data],
  );
  const escreventes = todasSkills.filter((skill) => skill.escrita);
  const consultivas = todasSkills.filter((skill) => !skill.escrita);

  const escreventesFiltradas = filtrarPorBusca(escreventes, busca);
  const consultivasFiltradas = filtrarPorBusca(consultivas, busca);
  const semResultadoDeBusca =
    todasSkills.length > 0 &&
    escreventesFiltradas.length === 0 &&
    consultivasFiltradas.length === 0;

  const cartaoDaSkill = (skill: SkillResumo, variante: 'completo' | 'compacto') => (
    <SkillCard
      key={skill.nome}
      skill={skill}
      variante={variante}
      problemas={problemas.get(skill.caminho)}
      onAbrir={() => void navegar(caminhoDoEditor(skill.nome))}
      {...(podeCriar && { onDuplicar: () => setDialogo({ tipo: 'duplicar', origem: skill }) })}
      {...(podeExcluir && { onExcluir: () => setDialogo({ tipo: 'excluir', skill }) })}
    />
  );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="h5" component="h1">
            Skills
          </Typography>
          <Typography variant="body2" color="text.secondary">
            O que o concierge sabe fazer. Abra uma skill para ajustar o que ele fala e o passo a
            passo.
          </Typography>
        </Box>
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
      {skills.isSuccess && todasSkills.length === 0 && <EmptyState frase={FRASE_VAZIA} />}
      {skills.isSuccess && todasSkills.length > 0 && (
        <>
          <NumerosDasSkills
            skills={todasSkills}
            problemas={problemas}
            podeVerProblemas={podeVerProblemas}
            arvore={arvore}
          />
          <TextField
            label="Buscar skill"
            slotProps={{ htmlInput: { 'aria-label': 'Buscar skill' } }}
            size="small"
            value={busca}
            onChange={(evento) => setBusca(evento.target.value)}
            sx={{ width: 300 }}
          />
          <GrupoDeSkills
            titulo="Registram no sistema"
            subtitulo="pedem confirmação do morador"
            colunas={2}
            skills={escreventesFiltradas}
            renderCartao={(skill) => cartaoDaSkill(skill, 'completo')}
          />
          <GrupoDeSkills
            titulo="Consultas e dúvidas"
            subtitulo="só leem, não gravam nada"
            colunas={3}
            skills={consultivasFiltradas}
            renderCartao={(skill) => cartaoDaSkill(skill, 'compacto')}
          />
          {semResultadoDeBusca && <EmptyState frase="Nenhuma skill encontrada." />}
        </>
      )}

      <DialogosDeSkills dialogo={dialogo} fechar={fechar} />
    </Box>
  );
}

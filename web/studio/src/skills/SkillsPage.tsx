import AddIcon from '@mui/icons-material/Add';
import { Box, Button, CircularProgress, Typography } from '@mui/material';
import { type ComponentProps, type ReactNode, useMemo, useState } from 'react';
import { useNavigate } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { useArvore } from '@/avancado/contratos';
import { skillsComAtualizacao } from '@/camadas/atualizacoesAbertas';
import { useAtualizacoes } from '@/camadas/contratos';
import { PecaAoCopiloto } from '@/copiloto/PecaAoCopiloto';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { PESOS, RAIO } from '@/theme/tokens';
import { type SkillResumo, useSkills } from './contratos';
import { caminhoDoEditor, type Dialogo, DialogosDeSkills } from './DialogosDeSkills';
import { type Problemas, problemasPorCaminho } from './estatisticas';
import { GrupoDeSkills } from './GrupoDeSkills';
import { NumerosDasSkills } from './NumerosDasSkills';
import { SkillCard } from './SkillCard';

const FRASE_VAZIA =
  'Uma skill é algo que o concierge sabe fazer pelo morador, como reservar o salão ou mandar a 2ª via do boleto.';

const SEM_PROBLEMAS: ReadonlyMap<string, Problemas> = new Map();
const SEM_ATUALIZACAO: ReadonlySet<string> = new Set();

type AbrirDialogo = (dialogo: Dialogo) => void;

export function SkillsPage(): ReactNode {
  const [dialogo, setDialogo] = useState<Dialogo | null>(null);
  useContextoDaTela({ tela: 'skills' });
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <CabecalhoDaPagina onNova={() => setDialogo({ tipo: 'nova' })} />
      <EstadoDasSkills onDialogo={setDialogo} />
      <DialogosDeSkills dialogo={dialogo} fechar={() => setDialogo(null)} />
    </Box>
  );
}

function CabecalhoDaPagina({ onNova }: { onNova: () => void }): ReactNode {
  const podeCriar = usePodeEditar('skills.criar');
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      <Box sx={{ flex: 1 }}>
        <Typography variant="h4" component="h1" sx={{ color: 'secondary.main' }}>
          Casos de uso
        </Typography>
        <Typography variant="subtitle1" sx={{ color: 'text.secondary', fontWeight: PESOS.regular }}>
          O que o concierge sabe fazer. Abra uma skill para ajustar o que ele fala e o passo a
          passo.
        </Typography>
      </Box>
      {podeCriar && (
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={onNova}
          sx={{ height: 42, px: 3, borderRadius: RAIO.menu, fontWeight: PESOS.semi }}
        >
          Nova skill
        </Button>
      )}
    </Box>
  );
}

function useProblemasDasSkills() {
  const podeVerProblemas = usePodeEditar('skills.editar_fluxo');
  const arvore = useArvore({ enabled: podeVerProblemas });
  const problemas = useMemo(
    () => (podeVerProblemas ? problemasPorCaminho(arvore.data ?? []) : SEM_PROBLEMAS),
    [podeVerProblemas, arvore.data],
  );
  return { podeVerProblemas, arvore, problemas };
}

function EstadoDasSkills({ onDialogo }: { onDialogo: AbrirDialogo }): ReactNode {
  const skills = useSkills();
  const visaoDeProblemas = useProblemasDasSkills();
  if (skills.isPending) {
    return <CircularProgress aria-label="Carregando skills" sx={{ alignSelf: 'center' }} />;
  }
  if (skills.isError) {
    return (
      <ErroComRetry mensagem={skills.error.message} onTentarDeNovo={() => void skills.refetch()} />
    );
  }
  if (skills.data.skills.length === 0) {
    return <EmptyState frase={FRASE_VAZIA} acao={<PecaAoCopiloto />} />;
  }
  return (
    <SkillsEncontradas skills={skills.data.skills} onDialogo={onDialogo} {...visaoDeProblemas} />
  );
}

type PropsDasEncontradas = Pick<
  ComponentProps<typeof NumerosDasSkills>,
  'skills' | 'problemas' | 'podeVerProblemas' | 'arvore'
> & { onDialogo: AbrirDialogo };

function SkillsEncontradas({ onDialogo, ...numeros }: PropsDasEncontradas): ReactNode {
  const { skills, problemas } = numeros;
  // O selo é complemento: sem a lista de atualizações a página segue sem ele.
  const { data: atualizacoes } = useAtualizacoes();
  const comAtualizacao = atualizacoes ? skillsComAtualizacao(atualizacoes) : SEM_ATUALIZACAO;
  const escreventes = skills.filter((skill) => skill.escrita);
  const consultivas = skills.filter((skill) => !skill.escrita);
  const cartao = (variante: 'completo' | 'compacto') => (skill: SkillResumo) => (
    <CartaoDaSkill
      key={skill.nome}
      skill={skill}
      variante={variante}
      problemas={problemas.get(skill.caminho)}
      temAtualizacao={comAtualizacao.has(skill.nome)}
      onDialogo={onDialogo}
    />
  );

  return (
    <>
      <NumerosDasSkills {...numeros} />
      <GrupoDeSkills
        titulo="Registram no sistema"
        subtitulo="pedem confirmação do morador"
        colunas={2}
        skills={escreventes}
        renderCartao={cartao('completo')}
      />
      <GrupoDeSkills
        titulo="Consultas e dúvidas"
        subtitulo="só leem, não gravam nada"
        colunas={3}
        skills={consultivas}
        renderCartao={cartao('compacto')}
      />
      {escreventes.length === 0 && consultivas.length === 0 && (
        <EmptyState frase="Nenhuma skill encontrada." />
      )}
    </>
  );
}

type PropsDoCartao = {
  skill: SkillResumo;
  variante: 'completo' | 'compacto';
  problemas: Problemas | undefined;
  temAtualizacao: boolean;
  onDialogo: AbrirDialogo;
};

function CartaoDaSkill({
  skill,
  variante,
  problemas,
  temAtualizacao,
  onDialogo,
}: PropsDoCartao): ReactNode {
  const navegar = useNavigate();
  const podeCriar = usePodeEditar('skills.criar');
  const podeExcluir = usePodeEditar('skills.excluir');
  return (
    <SkillCard
      skill={skill}
      variante={variante}
      problemas={problemas}
      temAtualizacao={temAtualizacao}
      onAbrir={() => void navegar(caminhoDoEditor(skill.nome))}
      {...(podeCriar && { onDuplicar: () => onDialogo({ tipo: 'duplicar', origem: skill }) })}
      {...(podeExcluir && { onExcluir: () => onDialogo({ tipo: 'excluir', skill }) })}
    />
  );
}

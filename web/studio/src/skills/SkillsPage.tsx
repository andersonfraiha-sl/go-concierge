import AddIcon from '@mui/icons-material/Add';
import { Box, Button, CircularProgress, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useNavigate } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { useArvore } from '@/avancado/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { NomeDialog, type NomeEscolhido } from '@/shared/ui/NomeDialog';
import { CartaoDeNumero } from './CartaoDeNumero';
import {
  preencherTextosNovos,
  type SkillResumo,
  useCriarSkill,
  useDuplicarSkill,
  useExcluirSkill,
  useSkills,
} from './contratos';
import { filtrarPorBusca, problemasPorCaminho, quartaEstatistica } from './estatisticas';
import { NovaSkillDialog } from './NovaSkillDialog';
import { SkillCard } from './SkillCard';

const FRASE_VAZIA =
  'Nenhuma skill ainda. Uma skill é algo que o concierge sabe fazer pelo morador, como reservar uma área comum.';

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
  const [busca, setBusca] = useState('');

  const podeCriar = usePodeEditar('skills.criar');
  const podeExcluir = usePodeEditar('skills.excluir');
  const podeVerProblemas = usePodeEditar('skills.editar_fluxo');
  const arvore = useArvore({ enabled: podeVerProblemas });
  const fechar = () => setDialogo(null);

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

  const todasSkills = skills.data?.skills ?? [];
  const problemas = podeVerProblemas
    ? problemasPorCaminho(arvore.data ?? [])
    : new Map<string, { erros: number; avisos: number }>();
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
      {skills.isSuccess && todasSkills.length === 0 && <EmptyState frase={FRASE_VAZIA} />}
      {skills.isSuccess && todasSkills.length > 0 && (
        <>
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 2 }}>
            <CartaoDeNumero rotulo="Skills" valor={todasSkills.length} />
            <CartaoDeNumero rotulo="Registram no sistema" valor={escreventes.length} />
            <CartaoDeNumero rotulo="Consultas e dúvidas" valor={consultivas.length} />
            <CartaoDeNumero {...quartaEstatistica(todasSkills, problemas, podeVerProblemas)} />
          </Box>
          <TextField
            label="Buscar skill"
            size="small"
            value={busca}
            onChange={(evento) => setBusca(evento.target.value)}
            sx={{ width: 300 }}
          />
          {escreventesFiltradas.length > 0 && (
            <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <Typography component="h2" variant="overline" color="text.secondary">
                Registram no sistema
              </Typography>
              <Box
                sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 1.5 }}
              >
                {escreventesFiltradas.map((skill) => cartaoDaSkill(skill, 'completo'))}
              </Box>
            </Box>
          )}
          {consultivasFiltradas.length > 0 && (
            <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
              <Typography component="h2" variant="overline" color="text.secondary">
                Consultas e dúvidas
              </Typography>
              <Box
                sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 1.5 }}
              >
                {consultivasFiltradas.map((skill) => cartaoDaSkill(skill, 'compacto'))}
              </Box>
            </Box>
          )}
          {semResultadoDeBusca && <EmptyState frase="Nenhuma skill encontrada." />}
        </>
      )}

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
    </Box>
  );
}

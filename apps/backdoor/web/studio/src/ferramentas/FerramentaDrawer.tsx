import { zodResolver } from '@hookform/resolvers/zod';
import CloseIcon from '@mui/icons-material/Close';
import { Alert, Box, Button, Chip, Drawer, IconButton, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { type Control, useForm, useWatch } from 'react-hook-form';
import type { Ferramenta } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import type { SkillResumo } from '@/skills/contratos';
import { GAVETA_DA_FERRAMENTA, PERIGO_SUAVE, PESOS, RAIO, TIPOS } from '@/theme/tokens';
import {
  ferramentaNova,
  lerAmostra,
  mantidaFora,
  paraGravar,
  removerFerramenta,
  salvarFerramenta,
  skillsQueUsam,
  tituloDe,
} from './contratos';
import {
  criarEsquemaDoFormulario,
  recursoCompleto,
  type Valores,
  valoresIniciais,
} from './formularioDaFerramenta';
import { SecaoDoDrawer } from './SecaoDoDrawer';
import { type Secao, secoesDoFormulario } from './secoesDoFormulario';

type Props = {
  /** `null` abre o formulário de uma ferramenta nova. */
  ferramenta: Ferramenta | null;
  skills: readonly SkillResumo[];
  podeEditar: boolean;
  onFechar: () => void;
};

const AVISO_FORA_DO_STUDIO = 'Esta ferramenta é mantida fora do Studio.';

const estaForaDoStudio = (ferramenta: Ferramenta | null, recusadaPeloServidor: boolean): boolean =>
  recusadaPeloServidor || (ferramenta !== null && mantidaFora(ferramenta));

/** Contagem curta ao lado do nome de cada seção, como no desenho aprovado. */
function useBadgesDasSecoes(control: Control<Valores>): Partial<Record<Secao, string>> {
  const recurso = useWatch({ control, name: 'recurso' });
  return {
    Parâmetros: String(recurso.params.length),
    Saída: `${recurso.saida.length} campos`,
    ...(recurso.conexao.metodo && { Conexão: recurso.conexao.metodo }),
    'Ajustes por condomínio': String(recurso.definicoes.length),
  };
}

function useSecoesAbertas(): [ReadonlySet<Secao>, (secao: Secao) => () => void] {
  const [abertas, setAbertas] = useState<ReadonlySet<Secao>>(() => new Set<Secao>(['Básico']));
  const alternar = (secao: Secao) => () =>
    setAbertas((atuais) => {
      const novas = new Set(atuais);
      if (!novas.delete(secao)) novas.add(secao);
      return novas;
    });
  return [abertas, alternar];
}

type Gravacao = {
  onFechar: () => void;
  onRecusada: () => void;
  onErro: (mensagem: string) => void;
};

function useGravacoes({ onFechar, onRecusada, onErro }: Gravacao) {
  const clienteDeConsultas = useQueryClient();
  const { mostrar } = useSnackbar();
  const aoConcluir =
    (mensagem: string) => (catalogo: Awaited<ReturnType<typeof salvarFerramenta>>) => {
      clienteDeConsultas.setQueryData(['ferramentas'], catalogo);
      mostrar(mensagem);
      onFechar();
    };
  const aoFalhar = (erro: Error) => {
    if (erro instanceof ApiError && erro.status === 409) return onRecusada();
    onErro(erro.message);
  };
  const salvar = useMutation({
    mutationFn: salvarFerramenta,
    onSuccess: aoConcluir('Ferramenta salva.'),
    onError: aoFalhar,
  });
  const remover = useMutation({
    mutationFn: removerFerramenta,
    onSuccess: aoConcluir('Ferramenta removida.'),
    onError: aoFalhar,
  });
  return { salvar, remover };
}

type RodapeProps = {
  podeRemover: boolean;
  travado: boolean;
  salvando: boolean;
  onRemover: () => void;
  onFechar: () => void;
};

function RodapeDoDrawer({ podeRemover, travado, salvando, onRemover, onFechar }: RodapeProps) {
  return (
    <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
      {podeRemover && (
        <Button
          variant="outlined"
          disabled={travado}
          onClick={onRemover}
          sx={{
            mr: 'auto',
            borderColor: PERIGO_SUAVE.borda,
            color: PERIGO_SUAVE.texto,
            fontWeight: PESOS.medio,
          }}
        >
          Remover
        </Button>
      )}
      <Button
        variant="outlined"
        onClick={onFechar}
        sx={{ borderColor: 'border.default', color: 'text.strong', fontWeight: PESOS.medio }}
      >
        Fechar
      </Button>
      <Button
        type="submit"
        variant="contained"
        disabled={travado || salvando}
        sx={{ fontWeight: PESOS.semi }}
      >
        Salvar
      </Button>
    </Box>
  );
}

type CabecalhoProps = {
  ferramenta: Ferramenta | null;
  usadaEm: readonly SkillResumo[];
  onFechar: () => void;
};

function CabecalhoDoDrawer({ ferramenta, usadaEm, onFechar }: CabecalhoProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
      <Box sx={{ flex: 1 }}>
        <Typography
          component="p"
          sx={{
            fontSize: TIPOS.overline,
            fontWeight: PESOS.semi,
            color: 'text.tertiary',
            textTransform: 'uppercase',
            letterSpacing: '.06em',
          }}
        >
          Ferramenta
        </Typography>
        <Typography variant="h5" component="h2" sx={{ color: 'secondary.main', mt: 1 }}>
          {ferramenta ? tituloDe(ferramenta) : 'Nova ferramenta'}
        </Typography>
        {usadaEm.length > 0 && (
          <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
            {usadaEm.map((skill) => (
              <Chip
                key={skill.nome}
                size="small"
                label={`Usada em ${skill.titulo}`}
                sx={{
                  borderRadius: RAIO.pilula,
                  bgcolor: 'background.neutralFill',
                  color: 'text.secondary',
                  fontSize: TIPOS.overline,
                  fontWeight: PESOS.medio,
                }}
              />
            ))}
          </Box>
        )}
      </Box>
      <IconButton
        aria-label="Fechar"
        onClick={onFechar}
        sx={{
          bgcolor: 'background.neutralFill',
          color: 'text.secondary',
          borderRadius: RAIO.campo,
        }}
      >
        <CloseIcon fontSize="small" />
      </IconButton>
    </Box>
  );
}

export function FerramentaDrawer({ ferramenta, skills, podeEditar, onFechar }: Props): ReactNode {
  const original = ferramenta?.name ?? '';
  const inicial = ferramenta ?? ferramentaNova();
  const [recusadaPeloServidor, setRecusadaPeloServidor] = useState(false);
  const [confirmandoRemocao, setConfirmandoRemocao] = useState(false);
  const [secoesAbertas, alternarSecao] = useSecoesAbertas();
  const { control, handleSubmit, setError, formState } = useForm({
    resolver: zodResolver(criarEsquemaDoFormulario(original)),
    defaultValues: valoresIniciais(inicial, ferramenta),
  });
  const { salvar, remover } = useGravacoes({
    onFechar,
    onRecusada: () => setRecusadaPeloServidor(true),
    onErro: (message) => setError('root', { message }),
  });

  const foraDoStudio = estaForaDoStudio(ferramenta, recusadaPeloServidor);
  const travado = !podeEditar || foraDoStudio;
  const secoes = secoesDoFormulario(control, travado, inicial.kind);
  const mensagemDeErro = formState.errors.root?.message;
  const usadaEm = ferramenta ? skillsQueUsam(skills, ferramenta.name) : [];
  const badges = useBadgesDasSecoes(control);

  const enviar = handleSubmit(({ recurso, textoAmostra }) => {
    const amostra = lerAmostra(textoAmostra);
    if (!amostra.ok) return;
    salvar.mutate({ original, recurso: paraGravar(recursoCompleto(inicial, recurso), amostra) });
  });

  return (
    <Drawer
      anchor="right"
      open
      onClose={onFechar}
      slotProps={{
        paper: {
          sx: { width: GAVETA_DA_FERRAMENTA, borderLeft: 1, borderColor: 'border.default' },
          role: 'dialog',
          'aria-label': 'Ferramenta',
        },
      }}
    >
      <Box
        component="form"
        noValidate
        onSubmit={(evento) => void enviar(evento)}
        sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2, overflow: 'auto' }}
      >
        <CabecalhoDoDrawer ferramenta={ferramenta} usadaEm={usadaEm} onFechar={onFechar} />
        {foraDoStudio && <Alert severity="info">{AVISO_FORA_DO_STUDIO}</Alert>}
        {mensagemDeErro && <Alert severity="error">{mensagemDeErro}</Alert>}

        {secoes.map(({ titulo, conteudo }) => (
          <SecaoDoDrawer
            key={titulo}
            titulo={titulo}
            badge={badges[titulo]}
            aberta={secoesAbertas.has(titulo)}
            onAlternar={alternarSecao(titulo)}
          >
            {conteudo}
          </SecaoDoDrawer>
        ))}

        <RodapeDoDrawer
          podeRemover={ferramenta !== null}
          travado={travado}
          salvando={salvar.isPending}
          onRemover={() => setConfirmandoRemocao(true)}
          onFechar={onFechar}
        />
      </Box>

      {ferramenta && (
        <ConfirmDialog
          aberto={confirmandoRemocao}
          titulo="Remover ferramenta"
          texto={`Remover a ferramenta "${tituloDe(ferramenta)}"? As skills que usam deixam de achá-la.`}
          confirmar="Remover"
          onCancelar={() => setConfirmandoRemocao(false)}
          onConfirmar={() => {
            setConfirmandoRemocao(false);
            remover.mutate(original);
          }}
        />
      )}
    </Drawer>
  );
}

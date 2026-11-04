import { zodResolver } from '@hookform/resolvers/zod';
import { Alert, Box, Button, Drawer, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { useForm } from 'react-hook-form';
import type { Ferramenta } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import {
  ferramentaNova,
  lerAmostra,
  mantidaFora,
  paraGravar,
  removerFerramenta,
  salvarFerramenta,
  tituloDe,
} from './contratos';
import {
  criarEsquemaDoFormulario,
  recursoCompleto,
  valoresIniciais,
} from './formularioDaFerramenta';
import { SecaoDoDrawer } from './SecaoDoDrawer';
import { type Secao, secoesDoFormulario } from './secoesDoFormulario';

type Props = {
  /** `null` abre o formulário de uma ferramenta nova. */
  ferramenta: Ferramenta | null;
  podeEditar: boolean;
  onFechar: () => void;
};

const AVISO_FORA_DO_STUDIO = 'Esta ferramenta é mantida fora do Studio.';

const estaForaDoStudio = (ferramenta: Ferramenta | null, recusadaPeloServidor: boolean): boolean =>
  recusadaPeloServidor || (ferramenta !== null && mantidaFora(ferramenta));

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
        <Button color="error" disabled={travado} onClick={onRemover} sx={{ mr: 'auto' }}>
          Remover
        </Button>
      )}
      <Button onClick={onFechar}>Fechar</Button>
      <Button type="submit" variant="contained" disabled={travado || salvando}>
        Salvar
      </Button>
    </Box>
  );
}

export function FerramentaDrawer({ ferramenta, podeEditar, onFechar }: Props): ReactNode {
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
        paper: { sx: { width: 'min(560px, 100vw)' }, role: 'dialog', 'aria-label': 'Ferramenta' },
      }}
    >
      <Box
        component="form"
        noValidate
        onSubmit={(evento) => void enviar(evento)}
        sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2, overflow: 'auto' }}
      >
        <Typography variant="h6" component="h2">
          {ferramenta ? tituloDe(ferramenta) : 'Nova ferramenta'}
        </Typography>
        {foraDoStudio && <Alert severity="info">{AVISO_FORA_DO_STUDIO}</Alert>}
        {mensagemDeErro && <Alert severity="error">{mensagemDeErro}</Alert>}

        {secoes.map(({ titulo, conteudo }) => (
          <SecaoDoDrawer
            key={titulo}
            titulo={titulo}
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

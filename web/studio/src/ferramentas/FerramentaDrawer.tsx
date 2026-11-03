import { zodResolver } from '@hookform/resolvers/zod';
import { Alert, Box, Button, Drawer, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { useForm } from 'react-hook-form';
import type { Ferramenta } from '@/catalogo/contratos';
import { ApiError } from '@/shared/api/http';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { CamposDeAjustes } from './CamposDeAjustes';
import { CamposDeConexao } from './CamposDeConexao';
import { CamposDeEntradaESaida } from './CamposDeEntradaESaida';
import { CamposDeIdentificacao } from './CamposDeIdentificacao';
import {
  ferramentaNova,
  lerAmostra,
  mantidaFora,
  paraGravar,
  removerFerramenta,
  salvarFerramenta,
  tituloDe,
} from './contratos';
import { FormularioSchema, recursoCompleto, valoresIniciais } from './formularioDaFerramenta';

type Props = {
  /** `null` abre o formulário de uma ferramenta nova. */
  ferramenta: Ferramenta | null;
  podeEditar: boolean;
  onFechar: () => void;
};

const AVISO_FORA_DO_STUDIO = 'Esta ferramenta é mantida fora do Studio.';

export function FerramentaDrawer({ ferramenta, podeEditar, onFechar }: Props): ReactNode {
  const clienteDeConsultas = useQueryClient();
  const { mostrar } = useSnackbar();
  const original = ferramenta?.name ?? '';
  const inicial = ferramenta ?? ferramentaNova();
  const [recusadaPeloServidor, setRecusadaPeloServidor] = useState(false);
  const [confirmandoRemocao, setConfirmandoRemocao] = useState(false);
  const { control, handleSubmit, setError, formState } = useForm({
    resolver: zodResolver(FormularioSchema),
    defaultValues: valoresIniciais(inicial, ferramenta),
  });
  const mensagemDeErro = formState.errors.root?.message;

  const foraDoStudio = (ferramenta !== null && mantidaFora(ferramenta)) || recusadaPeloServidor;
  const travado = !podeEditar || foraDoStudio;

  const aoConcluir =
    (mensagem: string) => (catalogo: Awaited<ReturnType<typeof salvarFerramenta>>) => {
      clienteDeConsultas.setQueryData(['ferramentas'], catalogo);
      mostrar(mensagem);
      onFechar();
    };
  const aoFalhar = (erro: Error) => {
    if (erro instanceof ApiError && erro.status === 409) return setRecusadaPeloServidor(true);
    setError('root', { message: erro.message });
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
        sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        <Typography variant="h6" component="h2">
          {ferramenta ? tituloDe(ferramenta) : 'Nova ferramenta'}
        </Typography>
        {foraDoStudio && <Alert severity="info">{AVISO_FORA_DO_STUDIO}</Alert>}
        {mensagemDeErro && <Alert severity="error">{mensagemDeErro}</Alert>}

        <CamposDeIdentificacao control={control} travado={travado} tipoInicial={inicial.kind} />
        <CamposDeEntradaESaida control={control} travado={travado} />
        <CamposDeConexao control={control} travado={travado} />
        <CamposDeAjustes control={control} travado={travado} />
        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
          {ferramenta && (
            <Button
              color="error"
              disabled={travado}
              onClick={() => setConfirmandoRemocao(true)}
              sx={{ mr: 'auto' }}
            >
              Remover
            </Button>
          )}
          <Button onClick={onFechar}>Fechar</Button>
          <Button type="submit" variant="contained" disabled={travado || salvar.isPending}>
            Salvar
          </Button>
        </Box>
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

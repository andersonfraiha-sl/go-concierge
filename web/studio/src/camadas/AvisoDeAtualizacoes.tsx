import { Alert, AlertTitle, Button } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { Link } from 'react-router';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { atualizacoesDaCamada, avisosDaCamada, descreverTrechos } from './atualizacoesAbertas';
import {
  type Atualizacoes,
  type Camada,
  CHAVE_DAS_ATUALIZACOES,
  marcarCiente,
  useAtualizacoes,
} from './contratos';
import { formatarData } from './formatarData';

type Resumo = Atualizacoes['atualizacoes'][number];
type Aviso = Atualizacoes['avisos'][number];

/** Faixas no topo do editor da camada: atualização esperando decisão e troca obrigatória já feita. */
export function AvisoDeAtualizacoes({ camada }: { camada: Camada }): ReactNode {
  const { data } = useAtualizacoes();
  // O aviso é um complemento: se a lista não vem, o editor segue sem ele.
  if (!data) return null;
  return (
    <>
      {avisosDaCamada(data, camada).map((aviso) => (
        <AvisoObrigatorio key={aviso.id} aviso={aviso} />
      ))}
      {atualizacoesDaCamada(data, camada).map((atualizacao) => (
        <FaixaDeAtualizacao key={atualizacao.id} atualizacao={atualizacao} />
      ))}
    </>
  );
}

function descricaoDaAtualizacao({ para, trechos, conflitos }: Resumo): string {
  const nova =
    para.resumo === ''
      ? `Nova versão ${para.numero}`
      : `Nova versão ${para.numero}: ${para.resumo}`;
  const contagem = trechos === 1 ? '1 trecho para revisar' : `${trechos} trechos para revisar`;
  const emConflito =
    conflitos === 0 ? '' : `, ${conflitos === 1 ? '1 em conflito' : `${conflitos} em conflito`}`;
  return `${nova}. ${contagem}${emConflito}.`;
}

function FaixaDeAtualizacao({ atualizacao }: { atualizacao: Resumo }): ReactNode {
  return (
    <Alert
      severity="info"
      action={
        <Button
          color="inherit"
          size="small"
          component={Link}
          to={`/atualizacoes/${encodeURIComponent(atualizacao.id)}`}
        >
          Revisar
        </Button>
      }
    >
      <AlertTitle>Essa skill tem atualizações</AlertTitle>
      {descricaoDaAtualizacao(atualizacao)}
    </Alert>
  );
}

function AvisoObrigatorio({ aviso }: { aviso: Aviso }): ReactNode {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  const ciente = useMutation({
    mutationFn: () => marcarCiente(aviso.id),
    onSuccess: () => cliente.invalidateQueries({ queryKey: CHAVE_DAS_ATUALIZACOES }),
    onError: (erro) => mostrar(mensagemDe(erro), 'error'),
  });
  return (
    <Alert
      severity="warning"
      action={
        <Button
          color="inherit"
          size="small"
          disabled={ciente.isPending}
          onClick={() => ciente.mutate()}
        >
          Entendi
        </Button>
      }
    >
      A Superlógica trocou {descreverTrechos(aviso.trechos)}: {aviso.motivo}
      {aviso.encerraEm && ` Esta skill para de funcionar em ${formatarData(aviso.encerraEm)}.`}
    </Alert>
  );
}

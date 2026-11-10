import { Alert, Box, Button, CircularProgress, Paper, Typography } from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { problemasDaRecusa } from '@/skills/abas/fluxoDaSkill';
import {
  type Camada,
  CHAVE_DAS_ATUALIZACOES,
  chaveDaCamada,
  chaveDoHistorico,
  type OrigemDeVersao,
  useHistorico,
  type VersaoDoHistorico,
  voltarVersao,
} from './contratos';
import { formatarData } from './formatarData';

const ORIGEM_EM_PORTUGUES: Record<OrigemDeVersao, string> = {
  importacao: 'Importação inicial',
  inicial: 'Versão inicial',
  publicacao: 'Publicação',
  decisao: 'Decisão sobre uma atualização',
  obrigatorio: 'Troca obrigatória',
  prazo: 'Atualização automática por prazo',
  volta: 'Volta a uma versão anterior',
};

type Props = {
  camada: Camada;
  /** Chamado depois que a volta foi gravada e a camada foi relida: o editor recomeça do que o servidor tem. */
  onVoltou: () => void;
};

export function HistoricoDaCamada({ camada, onVoltou }: Props): ReactNode {
  const historico = useHistorico(camada);
  const podeVoltar = usePodeEditarCondominio('skills.editar_textos');
  if (historico.isPending) return <CircularProgress aria-label="Carregando histórico" />;
  if (historico.isError) {
    return (
      <ErroComRetry
        mensagem={historico.error.message}
        onTentarDeNovo={() => void historico.refetch()}
      />
    );
  }
  const { versoes } = historico.data;
  if (versoes.length === 0) return <EmptyState frase="Esta versão ainda não tem histórico." />;
  const atual = Math.max(...versoes.map((versao) => versao.numero));
  return (
    <ListaDeVersoes
      camada={camada}
      versoes={versoes}
      onVoltou={onVoltou}
      podeVoltarPara={(numero) => podeVoltar && numero !== atual}
    />
  );
}

type PropsDaLista = Props & {
  versoes: readonly VersaoDoHistorico[];
  podeVoltarPara: (numero: number) => boolean;
};

function ListaDeVersoes({ camada, versoes, onVoltou, podeVoltarPara }: PropsDaLista): ReactNode {
  const [escolhida, setEscolhida] = useState<number | null>(null);
  const voltar = useVoltar(camada, onVoltou);
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      {voltar.isError && <Alert severity="error">{textoDoErro(voltar.error)}</Alert>}
      <Box
        component="ul"
        sx={{ m: 0, p: 0, listStyle: 'none', display: 'flex', flexDirection: 'column', gap: 1.5 }}
      >
        {versoes.map((versao) => (
          <VersaoDaLista
            key={versao.numero}
            versao={versao}
            onVoltar={podeVoltarPara(versao.numero) ? () => setEscolhida(versao.numero) : undefined}
          />
        ))}
      </Box>
      <ConfirmDialog
        aberto={escolhida !== null}
        titulo={`Voltar para a versão ${escolhida}?`}
        texto="Uma versão nova é criada com o conteúdo dela."
        confirmar="Voltar"
        onCancelar={() => setEscolhida(null)}
        onConfirmar={() => {
          if (escolhida !== null) voltar.mutate(escolhida);
          setEscolhida(null);
        }}
      />
    </Box>
  );
}

function textoDoErro(erro: unknown): string {
  const problemas = problemasDaRecusa(erro);
  if (problemas === null) return mensagemDe(erro);
  return `Não dá para voltar: ${problemas.map((problema) => problema.mensagem).join(' ')}`;
}

function useVoltar(camada: Camada, onVoltou: () => void) {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  return useMutation({
    mutationFn: async (numero: number) => ({ numero, criada: await voltarVersao(camada, numero) }),
    onSuccess: async ({ numero, criada }) => {
      await Promise.all(
        [chaveDaCamada(camada), chaveDoHistorico(camada), CHAVE_DAS_ATUALIZACOES].map((queryKey) =>
          cliente.invalidateQueries({ queryKey }),
        ),
      );
      mostrar(`Versão ${criada.numero} criada com o conteúdo da versão ${numero}.`);
      onVoltou();
    },
  });
}

type Decisao = NonNullable<VersaoDoHistorico['decisao']>;

function textoDaDecisao({ para, aceitos, mantidos, misturados }: Decisao): string {
  const partes = [
    aceitos > 0 && `aceitou ${aceitos}`,
    mantidos > 0 && `manteve ${mantidos}`,
    misturados > 0 && `misturou ${misturados}`,
  ].filter((parte) => parte !== false);
  return `Sobre a versão ${para}: ${partes.join(', ')}`;
}

type PropsDaVersao = { versao: VersaoDoHistorico; onVoltar: (() => void) | undefined };

function VersaoDaLista({ versao, onVoltar }: PropsDaVersao): ReactNode {
  return (
    <Paper
      component="li"
      variant="outlined"
      aria-label={`Versão ${versao.numero}`}
      sx={{ p: 2, display: 'flex', gap: 2, alignItems: 'flex-start' }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="subtitle2" component="h3">
          Versão {versao.numero}
        </Typography>
        <Typography variant="caption" color="textSecondary">
          {formatarData(versao.criadoEm)} · {versao.autor} · {ORIGEM_EM_PORTUGUES[versao.origem]}
        </Typography>
        {versao.resumo !== '' && <Typography>{versao.resumo}</Typography>}
        {versao.motivo !== '' && <Typography variant="body2">Motivo: {versao.motivo}</Typography>}
        {versao.decisao && (
          <Typography variant="body2">{textoDaDecisao(versao.decisao)}</Typography>
        )}
      </Box>
      {onVoltar && (
        <Button variant="outlined" size="small" onClick={onVoltar}>
          Voltar para esta versão
        </Button>
      )}
    </Paper>
  );
}

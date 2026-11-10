import { Box, CircularProgress, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { useParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { CabecalhoDeDetalhe } from '@/app/detalhe/CabecalhoDeDetalhe';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { AvisosDaComparacao } from './AvisosDaComparacao';
import { BarraDeDecisao } from './BarraDeDecisao';
import { rotaDaCamada } from './camadaDaRota';
import {
  type Atualizacao,
  type Camada,
  chaveDaAtualizacao,
  type EstadoDeAtualizacao,
  useAtualizacao,
  useCamada,
} from './contratos';
import { aceitarTudo, escolhasIniciais, pendentes, recusarTudo } from './decisoes';
import { classificarErroDaDecisao, distribuirProblemas } from './erroDaDecisao';
import { formatarData } from './formatarData';
import type { RotulosDasEtapas } from './leituraDoTrecho';
import { useOpcoesDeCamada } from './SeletorDeCamada';
import { TrechoDaComparacao } from './TrechoDaComparacao';
import { useDecisao } from './useDecisao';

const ESTADOS_QUE_ACEITAM_DECISAO: readonly EstadoDeAtualizacao[] = [
  'pendente',
  'recusada',
  'parcial',
];

export function ComparacaoPage(): ReactNode {
  const { id } = useParams();
  if (!id) return <EmptyState frase="Não encontramos esta atualização." />;
  return <CarregadorDaComparacao id={id} />;
}

function CarregadorDaComparacao({ id }: { id: string }): ReactNode {
  const lida = useAtualizacao(id);
  if (lida.isPending) {
    return (
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
        <CircularProgress aria-label="Carregando atualização" />
      </Box>
    );
  }
  if (lida.isError) {
    return (
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
        <ErroComRetry mensagem={lida.error.message} onTentarDeNovo={() => void lida.refetch()} />
      </Box>
    );
  }
  // A chave é a versão vista: recarregar com a cabeça nova recomeça as escolhas do zero.
  return <Comparacao key={lida.data.cabeca} atualizacao={lida.data} />;
}

function rotulosDasEtapas(atualizacao: Atualizacao): RotulosDasEtapas {
  return new Map(
    atualizacao.diferencas.filter((d) => d.tipo === 'etapa').map((d) => [d.trecho, d.rotulo]),
  );
}

function CabecalhoDaComparacao({ camada }: { camada: Camada }): ReactNode {
  const lida = useCamada(camada.skill, camada.tipo, camada.id);
  const nomeDaCamada = useOpcoesDeCamada(camada.skill).find(
    (opcao) => opcao.rota === rotaDaCamada(camada),
  )?.rotulo;
  return (
    <CabecalhoDeDetalhe
      voltar={{ para: rotaDaCamada(camada), rotulo: 'Voltar para a versão' }}
      titulo={`Atualização · ${lida.data?.documento.textos.titulo ?? camada.skill}`}
      subtitulo={nomeDaCamada}
    />
  );
}

function Comparacao({ atualizacao }: { atualizacao: Atualizacao }): ReactNode {
  const { id, camada, diferencas, estado, para, prazoEm } = atualizacao;
  const cliente = useQueryClient();
  const podeDecidir = usePodeEditarCondominio('skills.decidir_atualizacao');
  const editavel = podeDecidir && ESTADOS_QUE_ACEITAM_DECISAO.includes(estado);
  const [escolhas, setEscolhas] = useState(() => escolhasIniciais(diferencas));
  const decisao = useDecisao(atualizacao, escolhas);
  const erro = decisao.isError ? classificarErroDaDecisao(decisao.error) : null;
  const problemas = distribuirProblemas(
    erro?.tipo === 'problemas' ? erro.problemas : [],
    diferencas,
  );
  const rotulos = rotulosDasEtapas(atualizacao);
  const faltam = pendentes(diferencas, escolhas).length;

  return (
    <>
      <CabecalhoDaComparacao camada={camada} />
      <Box
        sx={{ ...ESTILO_DO_CONTEUDO_DE_DETALHE, display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        <Box>
          <Typography variant="h6" component="h2">
            Nova versão {para.numero}
            {para.resumo !== '' && `: ${para.resumo}`}
          </Typography>
          {prazoEm && (
            <Typography variant="body2" color="textSecondary">
              Chega sozinha em {formatarData(prazoEm)}. O que você não alterou entra nesse dia; o
              que você alterou espera a sua decisão.
            </Typography>
          )}
        </Box>
        <AvisosDaComparacao
          erro={erro}
          gerais={problemas.gerais}
          semPermissao={!podeDecidir}
          estado={estado}
          onRecarregar={() => void cliente.invalidateQueries({ queryKey: chaveDaAtualizacao(id) })}
        />
        {editavel && (
          <BarraDeDecisao
            faltam={faltam}
            enviando={decisao.isPending}
            onAceitarTudo={() => setEscolhas(aceitarTudo(diferencas))}
            onRecusarTudo={() => setEscolhas(recusarTudo(diferencas))}
            onSalvar={() => decisao.mutate()}
          />
        )}
        {diferencas.length === 0 && (
          <EmptyState frase="Esta atualização não tem trechos para revisar." />
        )}
        {diferencas.map((diferenca) => (
          <TrechoDaComparacao
            key={diferenca.trecho}
            diferenca={diferenca}
            camada={camada}
            versaoNova={para.numero}
            rotulosDasEtapas={rotulos}
            escolha={escolhas[diferenca.trecho]}
            problemas={problemas.porTrecho.get(diferenca.trecho) ?? []}
            editavel={editavel}
            onEscolher={(escolha) => setEscolhas({ ...escolhas, [diferenca.trecho]: escolha })}
          />
        ))}
      </Box>
    </>
  );
}

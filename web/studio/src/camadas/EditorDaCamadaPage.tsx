import { Box, CircularProgress } from '@mui/material';
import type { ReactNode } from 'react';
import { useParams, useSearchParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { CabecalhoDeDetalhe } from '@/app/detalhe/CabecalhoDeDetalhe';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { EmptyState } from '@/shared/ui/EmptyState';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { SectionCard } from '@/shared/ui/SectionCard';
import { QuandoUsarAba } from '@/skills/QuandoUsarAba';
import { camadaDaRota, rotaDaCamada, tipoDoSegmento } from './camadaDaRota';
import { type Camada, type CamadaLida, useCamada } from './contratos';
import { FluxoDaCamada } from './FluxoDaCamada';
import { PublicarCamada } from './PublicarCamada';
import { SeletorDeCamada, useOpcoesDeCamada } from './SeletorDeCamada';
import { type RascunhoDaCamada, useRascunhoDaCamada } from './useRascunhoDaCamada';

const ABAS = [
  { id: 'quando-usar', rotulo: 'Quando usar' },
  { id: 'fluxo', rotulo: 'Fluxo' },
  { id: 'orientacao', rotulo: 'Orientação' },
  { id: 'historico', rotulo: 'Histórico' },
] as const;
type AbaId = (typeof ABAS)[number]['id'];
const ABA_PADRAO: AbaId = 'quando-usar';

// A administradora não tem orientação própria: ela é do condomínio.
const abasDa = (camada: Camada) =>
  ABAS.filter((aba) => aba.id !== 'orientacao' || camada.tipo === 'condominio');

export function EditorDaCamadaPage(): ReactNode {
  const { nome, tipo, id } = useParams();
  const camada = camadaDaRota({ nome, tipo: tipoDoSegmento(tipo), id });
  if (!camada) return <EmptyState frase="Não encontramos esta versão." />;
  return <CarregadorDaCamada key={rotaDaCamada(camada)} camada={camada} />;
}

function CarregadorDaCamada({ camada }: { camada: Camada }): ReactNode {
  const lida = useCamada(camada.skill, camada.tipo, camada.id);
  if (lida.isPending) {
    return (
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
        <CircularProgress aria-label="Carregando versão" />
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
  return <EditorDaCamada camada={camada} lida={lida.data} />;
}

function EditorDaCamada({ camada, lida }: { camada: Camada; lida: CamadaLida }): ReactNode {
  const rascunho = useRascunhoDaCamada(camada, lida);
  const { mostrar } = useSnackbar();
  const podeEditarTextos = usePodeEditarCondominio('skills.editar_textos');
  const podeEditarEtapas = usePodeEditarCondominio('skills.editar_etapas');
  const [params, setParams] = useSearchParams();
  const abas = abasDa(camada);
  const aba = abas.find((candidata) => candidata.id === params.get('aba'))?.id ?? ABA_PADRAO;
  const nomeDaCamada = useOpcoesDeCamada(camada.skill).find(
    (opcao) => opcao.rota === rotaDaCamada(camada),
  )?.rotulo;
  const salvando = rascunho.salvamento.tipo === 'salvando';
  const comErro = rascunho.salvamento.tipo === 'erro';

  return (
    <>
      <CabecalhoDeDetalhe
        voltar={{
          para: `/skills/${encodeURIComponent(camada.skill)}`,
          rotulo: 'Voltar para a versão da Superlógica',
        }}
        titulo={`Personalizar · ${lida.documento.textos.titulo}`}
        subtitulo={nomeDaCamada}
        somenteLeitura={!podeEditarTextos && !podeEditarEtapas}
        acoes={
          <>
            <SeletorDeCamada skill={camada.skill} atual={camada} />
            {aba !== 'fluxo' && (
              <SaveIndicator
                estado={rascunho.salvamento}
                onTentarDeNovo={() => void rascunho.tentarDeNovo()}
              />
            )}
            <PublicarCamada
              desabilitado={salvando || comErro || !podeEditarTextos}
              onPublicar={async (resumo) => {
                await rascunho.publicar(resumo);
                mostrar('Versão publicada.');
              }}
            />
          </>
        }
        abas={{
          rotulo: 'Abas da versão',
          itens: abas,
          atual: aba,
          onTrocar: (proxima) => setParams({ aba: proxima }, { replace: true }),
        }}
      />
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
        <ConteudoDaAba
          aba={aba}
          camada={camada}
          lida={lida}
          rascunho={rascunho}
          podeEditarTextos={podeEditarTextos}
          podeEditarEtapas={podeEditarEtapas}
        />
      </Box>
    </>
  );
}

type ConteudoProps = {
  aba: AbaId;
  camada: Camada;
  lida: CamadaLida;
  rascunho: RascunhoDaCamada;
  podeEditarTextos: boolean;
  podeEditarEtapas: boolean;
};

function ConteudoDaAba({ aba, camada, lida, rascunho, ...pode }: ConteudoProps): ReactNode {
  switch (aba) {
    case 'quando-usar':
      return (
        <QuandoUsarAba
          nome={camada.skill}
          control={rascunho.controlDosTextos}
          editavel={pode.podeEditarTextos}
          comPainel={false}
        />
      );
    case 'fluxo':
      return lida.documento.textos.temFluxo ? (
        <FluxoDaCamada
          skill={camada.skill}
          fluxo={rascunho.fluxo}
          onMudarFluxo={rascunho.setFluxo}
          editavel={pode.podeEditarEtapas}
          ferramentasDaBase={lida.ferramentasDaBase}
          problemas={rascunho.problemas}
          salvamento={rascunho.salvamento}
          onTentarDeNovo={() => void rascunho.tentarDeNovo()}
        />
      ) : (
        <EmptyState frase="Esta versão não tem fluxo: ela responde pela base de conhecimento." />
      );
    case 'orientacao':
      return <OrientacaoDoCondominio rascunho={rascunho} editavel={pode.podeEditarEtapas} />;
    case 'historico':
      return <EmptyState frase="O histórico desta versão aparece aqui." />;
    default:
      return assertNever(aba);
  }
}

function OrientacaoDoCondominio({
  rascunho,
  editavel,
}: {
  rascunho: RascunhoDaCamada;
  editavel: boolean;
}): ReactNode {
  return (
    <SectionCard
      titulo="Orientação para este condomínio"
      ajuda="Um recado do condomínio para o concierge seguir nesta versão."
    >
      <CampoTexto
        control={rascunho.controlDaOrientacao}
        name="orientacao"
        multiline
        minRows={3}
        fullWidth
        disabled={!editavel}
        slotProps={{ htmlInput: { 'aria-label': 'Orientação para este condomínio' } }}
      />
    </SectionCard>
  );
}

function assertNever(valor: never): never {
  throw new Error(`aba não tratada: ${JSON.stringify(valor)}`);
}

import { Alert, Box, Button, CircularProgress } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useMemo, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { tituloDe } from '@/ferramentas/contratos';
import { FluxoCanvas } from '@/fluxo/canvas/FluxoCanvas';
import { novoBloco } from '@/fluxo/modelo/catalogo';
import { inserirBloco, moverBloco, removerBloco } from '@/fluxo/modelo/edicao';
import type { Fluxo, FluxoLido, TipoBloco } from '@/fluxo/modelo/tipos';
import { validarFluxo } from '@/fluxo/modelo/validar';
import { ApiError } from '@/shared/api/http';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { EmptyState } from '@/shared/ui/EmptyState';
import { tokens } from '@/theme/tokens';
import { chaveDoFluxo, salvarFluxo, useFluxo } from '../contratos';
import { BarraDoFluxo } from './BarraDoFluxo';
import {
  blocosRealcados,
  juntarProblemas,
  novoId,
  problemasDaRecusa,
  useCatalogoDeFerramentas,
} from './fluxoDaSkill';
import { PainelLateralDoFluxo } from './PainelLateralDoFluxo';

type Props = { nome: string; caminho: string };

// Título da skill, abas e margens do editor acima da aba; o resto da janela é do canvas.
const ALTURA_DO_CABECALHO_DO_EDITOR = 160;
const ALTURA_DA_AREA = `calc(100vh - ${tokens.layout.navbarHeight + ALTURA_DO_CABECALHO_DO_EDITOR}px)`;
const ALTURA_MINIMA_DA_AREA = 480;

const ehSemFluxo = (erro: Error): boolean =>
  erro instanceof ApiError && erro.status === 404 && erro.codigo === 'SEM_FLUXO';

export function FluxoAba({ nome }: Props): ReactNode {
  const lido = useFluxo(nome);
  useContextoDaTela({ tela: 'fluxo', skill: nome });
  if (lido.isPending) return <CircularProgress aria-label="Carregando fluxo" />;
  if (lido.isError) {
    if (ehSemFluxo(lido.error)) {
      return (
        <EmptyState frase="Esta skill não tem fluxo: ela responde pela base de conhecimento." />
      );
    }
    return (
      <Alert
        severity="error"
        action={<Button onClick={() => void lido.refetch()}>Tentar de novo</Button>}
      >
        {lido.error.message}
      </Alert>
    );
  }
  return <EditorDeFluxo key={nome} nome={nome} lido={lido.data} />;
}

type EditorProps = { nome: string; lido: FluxoLido };

function EditorDeFluxo({ nome, lido }: EditorProps): ReactNode {
  const [fluxo, setFluxo] = useState<Fluxo>(lido.fluxo);
  const [problemasDoServidor, setProblemasDoServidor] = useState(lido.problemas);
  const [selecionado, setSelecionado] = useState<string | null>(null);
  const editavel = usePodeEditar('skills.editar_fluxo');
  const cliente = useQueryClient();
  const { alvosRealcados } = useCopiloto();
  const realcados = useMemo(() => blocosRealcados(alvosRealcados, nome), [alvosRealcados, nome]);
  const catalogo = useCatalogoDeFerramentas();
  const ferramentas = catalogo.estado === 'pronto' ? catalogo.ferramentas : null;

  const locais = useMemo(() => validarFluxo(fluxo), [fluxo]);
  const problemas = useMemo(
    () => juntarProblemas(locais, problemasDoServidor, fluxo.blocos),
    [locais, problemasDoServidor, fluxo.blocos],
  );
  const tituloDaFerramenta = useMemo(() => {
    const titulos = new Map((ferramentas ?? []).map((f) => [f.name, tituloDe(f)]));
    return (recurso: string): string => titulos.get(recurso) ?? recurso;
  }, [ferramentas]);

  const sincronia = useEdicaoSobOServidor({
    servidor: lido.fluxo,
    alvos: [`skill:${nome}:fluxo`],
    versao: lido.versao,
    local: fluxo,
    resetar: setFluxo,
    salvar: async (valor, versao) => {
      try {
        const gravado = await salvarFluxo(nome, { versao, fluxo: valor });
        setProblemasDoServidor(gravado.problemas);
        // A aba sai da tela ao trocar de aba; ao voltar, o cache precisa ter a versão e o fluxo gravados.
        cliente.setQueryData<FluxoLido>(chaveDoFluxo(nome), {
          versao: gravado.versao,
          fluxo: valor,
          problemas: gravado.problemas,
        });
        return gravado.versao;
      } catch (erro) {
        const recusados = problemasDaRecusa(erro);
        if (recusados !== null) setProblemasDoServidor(recusados);
        throw erro;
      }
    },
  });
  const { estado, tentarDeNovo } = useAutosave({
    valor: fluxo,
    bloqueio: sincronia.bloqueio ?? (locais.length > 0 ? 'corrija o bloco marcado' : null),
    salvar: sincronia.salvar,
  });

  const inserir = (indice: number, tipo: TipoBloco) => {
    const id = novoId(fluxo.blocos);
    setFluxo((atual) => inserirBloco(atual, indice, novoBloco(tipo, id)));
    setSelecionado(id);
  };
  const excluir = (id: string) => {
    setFluxo((atual) => removerBloco(atual, id));
    setSelecionado(null);
  };

  const blocoSelecionado = fluxo.blocos.find((b) => b.id === selecionado);

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1,
        height: ALTURA_DA_AREA,
        minHeight: ALTURA_MINIMA_DA_AREA,
      }}
    >
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
      <BarraDoFluxo
        editavel={editavel}
        doFluxoTodo={problemas.filter((p) => p.blocoId === undefined)}
        salvamento={estado}
        onTentarDeNovo={() => void tentarDeNovo()}
      />
      <Box
        sx={{
          display: 'flex',
          flex: 1,
          minHeight: 0,
          border: 1,
          borderColor: 'divider',
          borderRadius: 1,
          overflow: 'hidden',
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <FluxoCanvas
            fluxo={fluxo}
            problemas={problemas}
            selecionado={selecionado}
            editavel={editavel}
            tituloDaFerramenta={tituloDaFerramenta}
            realcados={realcados}
            onSelecionar={setSelecionado}
            onInserir={inserir}
            onMover={(de, para) => setFluxo((atual) => moverBloco(atual, de, para))}
          />
        </Box>
        <PainelLateralDoFluxo
          fluxo={fluxo}
          bloco={blocoSelecionado}
          problemas={problemas}
          catalogo={catalogo}
          editavel={editavel}
          onMudarFluxo={setFluxo}
          onExcluir={excluir}
        />
      </Box>
    </Box>
  );
}

import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material';
import { useMemo, useRef, useState, type ReactNode } from 'react';
import { z } from 'zod';
import { useFerramentas, type Ferramenta } from '@/catalogo/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { FluxoCanvas } from '@/fluxo/canvas/FluxoCanvas';
import { novoBloco } from '@/fluxo/modelo/catalogo';
import { atualizarBloco, inserirBloco, moverBloco, removerBloco } from '@/fluxo/modelo/edicao';
import {
  ProblemaFluxoSchema,
  type Bloco,
  type Fluxo,
  type FluxoLido,
  type Informacao,
  type ProblemaFluxo,
  type TipoBloco,
} from '@/fluxo/modelo/tipos';
import { validarFluxo } from '@/fluxo/modelo/validar';
import { BlocoPainel } from '@/fluxo/painel/BlocoPainel';
import { InformacoesPainel } from '@/fluxo/painel/InformacoesPainel';
import { usePodeEditar } from '@/acesso/eu';
import { ApiError } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { EmptyState } from '@/shared/ui/EmptyState';
import { tokens } from '@/theme/tokens';
import { useFluxo, salvarFluxo } from '../contratos';

type Props = { nome: string; caminho: string };

const LARGURA_PAINEL = 360;
const SEM_FERRAMENTAS: readonly Ferramenta[] = [];

const ehSemFluxo = (erro: Error): boolean =>
  erro instanceof ApiError && erro.status === 404 && erro.codigo === 'SEM_FLUXO';

export function FluxoAba({ nome }: Props): ReactNode {
  const lido = useFluxo(nome);
  if (lido.isPending) return <CircularProgress aria-label="Carregando fluxo" />;
  if (lido.isError) {
    if (ehSemFluxo(lido.error)) {
      return <EmptyState frase="Esta skill não tem fluxo: ela responde pela base de conhecimento." />;
    }
    return (
      <Alert severity="error" action={<Button onClick={() => void lido.refetch()}>Tentar de novo</Button>}>
        {lido.error.message}
      </Alert>
    );
  }
  return <EditorDeFluxo key={nome} nome={nome} lido={lido.data} />;
}

const RecusaSchema = z.object({ problemas: listaDoGo(ProblemaFluxoSchema) });

/** Problemas que o servidor mandou junto do 422; null para qualquer outra falha. */
function problemasDaRecusa(erro: unknown): ProblemaFluxo[] | null {
  if (!(erro instanceof ApiError) || erro.status !== 422) return null;
  const lida = RecusaSchema.safeParse(erro.corpo);
  return lida.success ? lida.data.problemas : null;
}

const chaveDoProblema = (p: ProblemaFluxo): string => `${p.blocoId ?? ''}|${p.codigo}|${p.mensagem}`;

/** Os locais valem sempre; os do servidor, só os de blocos que ainda existem e que o local não repete. */
function juntarProblemas(locais: ProblemaFluxo[], doServidor: ProblemaFluxo[], blocos: readonly Bloco[]): ProblemaFluxo[] {
  const ids = new Set(blocos.map((b) => b.id));
  const jaVistos = new Set(locais.map(chaveDoProblema));
  const aindaValidos = doServidor.filter(
    (p) => p.blocoId !== undefined && ids.has(p.blocoId) && !jaVistos.has(chaveDoProblema(p)),
  );
  return [...locais, ...aindaValidos];
}

/** Primeiro `bN` livre: curto, legível e sem colidir com os IDs que vieram do servidor. */
function novoId(blocos: readonly Bloco[]): string {
  const usados = new Set(blocos.map((b) => b.id));
  let numero = blocos.length + 1;
  while (usados.has(`b${numero}`)) numero += 1;
  return `b${numero}`;
}

type EditorProps = { nome: string; lido: FluxoLido };

function EditorDeFluxo({ nome, lido }: EditorProps): ReactNode {
  const [fluxo, setFluxo] = useState<Fluxo>(lido.fluxo);
  const [problemasDoServidor, setProblemasDoServidor] = useState(lido.problemas);
  const [selecionado, setSelecionado] = useState<string | null>(null);
  const versao = useRef(lido.versao);
  const editavel = usePodeEditar('skills.editar_fluxo');
  const ferramentas = useFerramentas().data?.recursos ?? SEM_FERRAMENTAS;

  const locais = useMemo(() => validarFluxo(fluxo), [fluxo]);
  const problemas = useMemo(
    () => juntarProblemas(locais, problemasDoServidor, fluxo.blocos),
    [locais, problemasDoServidor, fluxo.blocos],
  );
  const tituloDaFerramenta = useMemo(() => {
    const titulos = new Map(ferramentas.map((f) => [f.name, tituloDe(f)]));
    return (recurso: string): string => titulos.get(recurso) ?? recurso;
  }, [ferramentas]);

  const { estado, tentarDeNovo } = useAutosave({
    valor: fluxo,
    bloqueio: locais.length > 0 ? 'corrija o bloco marcado' : null,
    salvar: async (valor) => {
      try {
        const gravado = await salvarFluxo(nome, { versao: versao.current, fluxo: valor });
        versao.current = gravado.versao;
        setProblemasDoServidor(gravado.problemas);
      } catch (erro) {
        const recusados = problemasDaRecusa(erro);
        if (recusados !== null) setProblemasDoServidor(recusados);
        throw erro;
      }
    },
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
  const mudarInformacoes = (informacoes: Informacao[]) => setFluxo((atual) => ({ ...atual, informacoes }));

  const blocoSelecionado = fluxo.blocos.find((b) => b.id === selecionado);
  const doFluxoTodo = problemas.filter((p) => p.blocoId === undefined);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1, height: 'calc(100vh - 220px)', minHeight: 480 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, minHeight: 32 }}>
        {doFluxoTodo.map((p) => (
          <Typography key={p.codigo} variant="caption" sx={{ color: tokens.color.studio.issueError }}>
            {p.mensagem}
          </Typography>
        ))}
        <Box sx={{ ml: 'auto' }}>
          <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
        </Box>
      </Box>
      <Box sx={{ display: 'flex', flex: 1, minHeight: 0, border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden' }}>
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <FluxoCanvas
            fluxo={fluxo}
            problemas={problemas}
            selecionado={selecionado}
            editavel={editavel}
            tituloDaFerramenta={tituloDaFerramenta}
            onSelecionar={setSelecionado}
            onInserir={inserir}
            onMover={(de, para) => setFluxo((atual) => moverBloco(atual, de, para))}
          />
        </Box>
        <Box
          component="aside"
          sx={{ width: LARGURA_PAINEL, flexShrink: 0, borderLeft: 1, borderColor: 'divider', overflowY: 'auto', bgcolor: 'background.paper' }}
        >
          {blocoSelecionado ? (
            <BlocoPainel
              key={blocoSelecionado.id}
              bloco={blocoSelecionado}
              problemas={problemas.filter((p) => p.blocoId === blocoSelecionado.id)}
              ferramentas={ferramentas}
              informacoes={fluxo.informacoes}
              editavel={editavel}
              onMudar={(bloco) => setFluxo((atual) => atualizarBloco(atual, bloco))}
              onExcluir={excluir}
            />
          ) : (
            <InformacoesPainel informacoes={fluxo.informacoes} editavel={editavel} onMudar={mudarInformacoes} />
          )}
        </Box>
      </Box>
    </Box>
  );
}

import { Box, Button, MenuItem, Tab, Tabs, TextField, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSkills } from '@/skills/contratos';
import { ArquivosAba } from './ArquivosAba';
import { achatar, type NoDaArvore, useArvore } from './contratos';
import { ImportarExportarAba } from './ImportarExportarAba';
import { OrquestracaoAba } from './OrquestracaoAba';
import { ProjecaoAba } from './ProjecaoAba';

type Contexto = { no: NoDaArvore; editar: boolean };

type Aba = {
  id: string;
  rotulo: string;
  /** Abas que falam de um item só mostram o seletor de item. */
  porItem: boolean;
  desenhar: (contexto: Contexto | null, editar: boolean) => ReactNode;
};

const ABAS: readonly Aba[] = [
  {
    id: 'arquivos',
    rotulo: 'Arquivos',
    porItem: true,
    desenhar: (c) => c && <ArquivosAba key={c.no.caminho} {...c} />,
  },
  {
    id: 'projecao',
    rotulo: 'Projeção',
    porItem: true,
    desenhar: (c) => c && <ProjecaoAba caminho={c.no.caminho} />,
  },
  {
    id: 'orquestracao',
    rotulo: 'Orquestração',
    porItem: false,
    desenhar: (_, editar) => <OrquestracaoAba editar={editar} />,
  },
  {
    id: 'importar',
    rotulo: 'Importar e exportar',
    porItem: true,
    desenhar: (c) => c && <ImportarExportarAba {...c} />,
  },
];

export function AvancadoPage(): ReactNode {
  const [params, setParams] = useSearchParams();
  const editar = usePodeEditar('studio.avancado');
  const arvore = useArvore();
  const skills = useSkills();
  const escolhido = params.get('item');

  const aba = ABAS.find((a) => a.id === params.get('aba')) ?? ABAS[0];
  if (!aba) return null;
  const nos = arvore.data ?? [];
  const no =
    nos.find((n) => n.caminho === escolhido) ??
    nos.find((n) => n.caminho === skills.data?.assistente) ??
    nos[0];

  const mudar = (chave: string, valor: string) =>
    setParams((atual) => new URLSearchParams({ ...Object.fromEntries(atual), [chave]: valor }), {
      replace: true,
    });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Typography variant="h5" component="h1">
        Avançado
      </Typography>
      <Tabs
        value={aba.id}
        onChange={(_, valor: string) => mudar('aba', valor)}
        aria-label="Seções do Avançado"
      >
        {ABAS.map((a) => (
          <Tab key={a.id} value={a.id} label={a.rotulo} />
        ))}
      </Tabs>
      {aba.porItem && (
        <SeletorDeItem nos={nos} atual={no} onEscolher={(caminho) => mudar('item', caminho)} />
      )}
      {aba.porItem && arvore.isPending && (
        <Typography color="text.secondary">Carregando…</Typography>
      )}
      {aba.porItem && arvore.isError && (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <Typography role="alert">Não foi possível carregar a lista.</Typography>
          <Button size="small" onClick={() => void arvore.refetch()}>
            Tentar de novo
          </Button>
        </Box>
      )}
      {aba.porItem && arvore.isSuccess && !no && (
        <EmptyState frase="Ainda não há assistente nem skills para mostrar." />
      )}
      {aba.desenhar(no ? { no, editar } : null, editar)}
    </Box>
  );
}

const contagem = (quantidade: number, singular: string, plural: string): string =>
  quantidade === 0 ? '' : ` · ${quantidade} ${quantidade === 1 ? singular : plural}`;

const descreverNo = (no: NoDaArvore): string =>
  `${no.caminho}${contagem(no.erros, 'erro', 'erros')}${contagem(no.avisos, 'aviso', 'avisos')}`;

type SeletorProps = {
  nos: NoDaArvore[];
  atual: NoDaArvore | undefined;
  onEscolher: (caminho: string) => void;
};

function SeletorDeItem({ nos, atual, onEscolher }: SeletorProps): ReactNode {
  if (!atual) return null;
  return (
    <TextField
      select
      size="small"
      label="Concierge ou skill"
      value={atual.caminho}
      onChange={(evento) => onEscolher(evento.target.value)}
      sx={{ maxWidth: 480 }}
    >
      {achatar(nos).map((no) => (
        <MenuItem key={no.caminho} value={no.caminho}>
          {descreverNo(no)}
        </MenuItem>
      ))}
    </TextField>
  );
}

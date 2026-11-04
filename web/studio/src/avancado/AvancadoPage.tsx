import { Box, Button, Chip, MenuItem, Tab, Tabs, TextField, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSkills } from '@/skills/contratos';
import { tokens } from '@/theme/tokens';
import { AjustesTecnicosAba } from './AjustesTecnicosAba';
import { ArquivosAba } from './ArquivosAba';
import { achatar, type NoDaArvore, useArvore } from './contratos';
import { IdentidadeAba } from './IdentidadeAba';
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
    id: 'identidade',
    rotulo: 'Identidade',
    porItem: true,
    desenhar: (c) => c && <IdentidadeAba key={c.no.caminho} {...c} />,
  },
  {
    id: 'projecao',
    rotulo: 'Prompt montado',
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
    id: 'ajustes',
    rotulo: 'Ajustes por condomínio (técnico)',
    porItem: false,
    desenhar: (_, editar) => <AjustesTecnicosAba editar={editar} />,
  },
  {
    id: 'importar',
    rotulo: 'Importar e exportar',
    porItem: true,
    desenhar: (c) => c && <ImportarExportarAba {...c} />,
  },
];

type Arvore = ReturnType<typeof useArvore>;

function useNoEscolhido(nos: NoDaArvore[], escolhido: string | null): NoDaArvore | undefined {
  const assistente = useSkills().data?.assistente;
  return (
    nos.find((candidato) => candidato.caminho === escolhido) ??
    nos.find((candidato) => candidato.caminho === assistente) ??
    nos[0]
  );
}

export function AvancadoPage(): ReactNode {
  const [params, setParams] = useSearchParams();
  const editar = usePodeEditar('studio.avancado');
  const arvore = useArvore();
  const nos = arvore.data ?? [];
  const noAtual = useNoEscolhido(nos, params.get('item'));

  const aba = ABAS.find((candidata) => candidata.id === params.get('aba')) ?? ABAS[0];
  if (!aba) return null;

  const mudar = (chave: string, valor: string) =>
    setParams((atual) => new URLSearchParams({ ...Object.fromEntries(atual), [chave]: valor }), {
      replace: true,
    });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <CabecalhoDoAvancado />
      <Tabs
        value={aba.id}
        onChange={(_, valor: string) => mudar('aba', valor)}
        aria-label="Seções do Avançado"
      >
        {ABAS.map((candidata) => (
          <Tab key={candidata.id} value={candidata.id} label={candidata.rotulo} />
        ))}
      </Tabs>
      {aba.porItem && (
        <>
          <SeletorDeItem
            nos={nos}
            atual={noAtual}
            onEscolher={(caminho) => mudar('item', caminho)}
          />
          <EstadoDaLista arvore={arvore} semItem={!noAtual} />
        </>
      )}
      {aba.desenhar(noAtual ? { no: noAtual, editar } : null, editar)}
    </Box>
  );
}

function CabecalhoDoAvancado(): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
      <Typography variant="h5" component="h1" sx={{ flex: 1 }}>
        Avançado
      </Typography>
      <Chip
        size="small"
        label="Mudanças aqui afetam todos os condomínios"
        sx={{
          bgcolor: tokens.color.warning.background,
          color: tokens.color.warning.text,
          fontWeight: 500,
        }}
      />
    </Box>
  );
}

function EstadoDaLista({ arvore, semItem }: { arvore: Arvore; semItem: boolean }): ReactNode {
  if (arvore.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (arvore.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível carregar a lista.</Typography>
        <Button size="small" onClick={() => void arvore.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }
  if (semItem) return <EmptyState frase="Ainda não há concierge nem skills para mostrar." />;
  return null;
}

const contagem = (quantidade: number, singular: string, plural: string): string =>
  quantidade === 0 ? '' : ` · ${quantidade} ${quantidade === 1 ? singular : plural}`;

const descreverNo = (item: NoDaArvore): string =>
  `${item.caminho}${contagem(item.erros, 'erro', 'erros')}${contagem(item.avisos, 'aviso', 'avisos')}`;

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
      {achatar(nos).map((item) => (
        <MenuItem key={item.caminho} value={item.caminho}>
          {descreverNo(item)}
        </MenuItem>
      ))}
    </TextField>
  );
}

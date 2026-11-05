import { Box, Button, MenuItem, TextField, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { type AbasDoDetalhe, CabecalhoDeDetalhe } from '@/app/detalhe/CabecalhoDeDetalhe';
import { EmptyState } from '@/shared/ui/EmptyState';
import { useSkills } from '@/skills/contratos';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';
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
    <>
      <CabecalhoDoAvancado
        abas={{
          rotulo: 'Seções do Avançado',
          itens: ABAS,
          atual: aba.id,
          onTrocar: (valor) => mudar('aba', valor),
        }}
      />
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          gap: 3,
          py: 3,
          px: 4,
          flex: 1,
          minHeight: 0,
        }}
      >
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
    </>
  );
}

/** Cabeçalho do Avançado; sem abas, serve também às telas de acesso negado e de erro. */
export function CabecalhoDoAvancado({ abas }: { abas?: AbasDoDetalhe<string> }): ReactNode {
  return (
    <CabecalhoDeDetalhe
      voltar={{ para: '/skills', rotulo: 'Voltar para Skills' }}
      titulo="Avançado"
      subtitulo="Arquivos e configurações internas · só para quem constrói skills"
      acoes={<AvisoDeAlcance />}
      {...(abas && { abas })}
    />
  );
}

function AvisoDeAlcance(): ReactNode {
  return (
    <Typography
      component="span"
      sx={{
        flexShrink: 0,
        py: 0.5,
        px: 1.25,
        borderRadius: RAIO.pilula,
        bgcolor: 'warning.soft',
        color: 'warning.onSoft',
        fontSize: TIPOS.caption,
        fontWeight: PESOS.medio,
      }}
    >
      Mudanças aqui afetam todos os condomínios
    </Typography>
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
  `${item.nome}${contagem(item.erros, 'erro', 'erros')}${contagem(item.avisos, 'aviso', 'avisos')}`;

type SeletorProps = {
  nos: NoDaArvore[];
  atual: NoDaArvore | undefined;
  onEscolher: (caminho: string) => void;
};

const ESTILO_DO_ROTULO_FIXO = {
  '& .MuiInputLabel-root.MuiInputLabel-shrink': {
    fontSize: TIPOS.overline,
    fontWeight: PESOS.semi,
    color: 'text.tertiary',
    textTransform: 'uppercase',
    letterSpacing: '.06em',
  },
} as const;

function SeletorDeItem({ nos, atual, onEscolher }: SeletorProps): ReactNode {
  if (!atual) return null;
  return (
    <TextField
      select
      size="small"
      label="Concierge ou skill"
      value={atual.caminho}
      onChange={(evento) => onEscolher(evento.target.value)}
      sx={{ maxWidth: 480, ...ESTILO_DO_ROTULO_FIXO }}
    >
      {achatar(nos).map((item) => (
        <MenuItem key={item.caminho} value={item.caminho}>
          {descreverNo(item)}
        </MenuItem>
      ))}
    </TextField>
  );
}

import AddIcon from '@mui/icons-material/Add';
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { type Ferramenta, useFerramentas } from '@/catalogo/contratos';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { EmptyState } from '@/shared/ui/EmptyState';
import { LARGURA_MINIMA_DO_CARTAO } from '@/theme/tokens';
import { CLASSES, type Classe, classeDe, GRUPOS, tituloDe } from './contratos';
import { FerramentaDrawer } from './FerramentaDrawer';

/** `null` = ferramenta nova; `undefined` = drawer fechado. */
type Aberta = Ferramenta | null | undefined;

const contemTexto = (ferramenta: Ferramenta, filtro: string): boolean =>
  [ferramenta.name, ferramenta.titulo, ferramenta.description].some((campo) =>
    campo?.toLowerCase().includes(filtro),
  );

function agruparPorClasse(ferramentas: readonly Ferramenta[]): [Classe, Ferramenta[]][] {
  return CLASSES.map((classe): [Classe, Ferramenta[]] => [
    classe,
    ferramentas.filter((ferramenta) => classeDe(ferramenta) === classe),
  ]).filter(([, grupo]) => grupo.length > 0);
}

type CabecalhoProps = { podeEditar: boolean; onNova: () => void };

function CabecalhoDaPagina({ podeEditar, onNova }: CabecalhoProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
      <Box sx={{ flex: 1 }}>
        <Typography variant="h5" component="h1">
          Ferramentas
        </Typography>
        <Typography color="text.secondary">
          Sistemas que as skills consultam e onde gravam.
        </Typography>
      </Box>
      {podeEditar && (
        <Button variant="contained" startIcon={<AddIcon />} onClick={onNova}>
          Nova ferramenta
        </Button>
      )}
    </Box>
  );
}

type ResultadoProps = {
  consulta: ReturnType<typeof useFerramentas>;
  filtro: string;
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function ResultadoDaConsulta({ consulta, filtro, podeEditar, onAbrir }: ResultadoProps): ReactNode {
  if (consulta.isPending) {
    return <CircularProgress aria-label="Carregando ferramentas" sx={{ alignSelf: 'center' }} />;
  }
  if (consulta.isError) {
    return (
      <Alert
        severity="error"
        action={
          <Button color="inherit" onClick={() => void consulta.refetch()}>
            Tentar de novo
          </Button>
        }
      >
        Não deu para carregar as ferramentas.
      </Alert>
    );
  }
  const texto = filtro.trim().toLowerCase();
  return (
    <Grupos
      ferramentas={consulta.data.recursos.filter((ferramenta) => contemTexto(ferramenta, texto))}
      temFiltro={filtro.trim() !== ''}
      podeEditar={podeEditar}
      onAbrir={onAbrir}
    />
  );
}

export function FerramentasPage(): ReactNode {
  const consulta = useFerramentas();
  const podeEditar = usePodeEditar('ferramentas.editar');
  const [filtro, setFiltro] = useState('');
  const [aberta, setAberta] = useState<Aberta>(undefined);
  useContextoDaTela({ tela: 'ferramentas' });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <CabecalhoDaPagina podeEditar={podeEditar} onNova={() => setAberta(null)} />
      <TextField
        label="Filtrar"
        size="small"
        value={filtro}
        onChange={(evento) => setFiltro(evento.target.value)}
        sx={{ maxWidth: 360 }}
      />
      <ResultadoDaConsulta
        consulta={consulta}
        filtro={filtro}
        podeEditar={podeEditar}
        onAbrir={setAberta}
      />
      {aberta !== undefined && (
        <FerramentaDrawer
          key={aberta?.name ?? 'nova'}
          ferramenta={aberta}
          podeEditar={podeEditar}
          onFechar={() => setAberta(undefined)}
        />
      )}
    </Box>
  );
}

type GruposProps = {
  ferramentas: readonly Ferramenta[];
  temFiltro: boolean;
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function Grupos({ ferramentas, temFiltro, podeEditar, onAbrir }: GruposProps): ReactNode {
  if (ferramentas.length === 0) {
    return (
      <EmptyState
        frase={temFiltro ? 'Nenhuma ferramenta com esse texto.' : 'Ainda não há ferramentas.'}
      />
    );
  }
  return agruparPorClasse(ferramentas).map(([classe, grupo]) => (
    <GrupoDeFerramentas
      key={classe}
      classe={classe}
      grupo={grupo}
      podeEditar={podeEditar}
      onAbrir={onAbrir}
    />
  ));
}

type GrupoProps = {
  classe: Classe;
  grupo: readonly Ferramenta[];
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function GrupoDeFerramentas({ classe, grupo, podeEditar, onAbrir }: GrupoProps): ReactNode {
  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Typography variant="subtitle1" component="h2">
        {GRUPOS[classe]}
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: `repeat(auto-fill, minmax(${LARGURA_MINIMA_DO_CARTAO}px, 1fr))`,
        }}
      >
        {grupo.map((ferramenta) => (
          <CartaoDeFerramenta
            key={ferramenta.name}
            ferramenta={ferramenta}
            clicavel={podeEditar}
            onAbrir={onAbrir}
          />
        ))}
      </Box>
    </Box>
  );
}

type ConteudoProps = { ferramenta: Ferramenta; comDescricao: boolean };

function ConteudoDoCartao({ ferramenta, comDescricao }: ConteudoProps): ReactNode {
  return (
    <CardContent sx={{ display: 'flex', flexDirection: 'column', gap: 1, height: '100%' }}>
      <Typography variant="subtitle2" component="h3">
        {tituloDe(ferramenta, comDescricao)}
      </Typography>
      {comDescricao && (
        <Typography variant="body2" color="text.secondary" sx={{ flex: 1 }}>
          {ferramenta.description}
        </Typography>
      )}
      {ferramenta.write && (
        <Chip
          size="small"
          color="warning"
          variant="outlined"
          label="Grava no sistema"
          sx={{ alignSelf: 'flex-start' }}
        />
      )}
    </CardContent>
  );
}

type CartaoProps = {
  ferramenta: Ferramenta;
  clicavel: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function CartaoDeFerramenta({ ferramenta, clicavel, onAbrir }: CartaoProps): ReactNode {
  // Quem não edita não vê a descrição técnica da ferramenta.
  const conteudo = <ConteudoDoCartao ferramenta={ferramenta} comDescricao={clicavel} />;
  return (
    <Card variant="outlined">
      {clicavel ? (
        <CardActionArea sx={{ height: '100%' }} onClick={() => onAbrir(ferramenta)}>
          {conteudo}
        </CardActionArea>
      ) : (
        conteudo
      )}
    </Card>
  );
}

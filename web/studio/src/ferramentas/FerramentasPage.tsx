import AddIcon from '@mui/icons-material/Add';
import SearchIcon from '@mui/icons-material/Search';
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  InputAdornment,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { type Ferramenta, useFerramentas } from '@/catalogo/contratos';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { EmptyState } from '@/shared/ui/EmptyState';
import { type SkillResumo, useSkills } from '@/skills/contratos';
import { ICONES, LARGURA_DA_LISTA_ESTREITA, PESOS, RAIO, TIPOS } from '@/theme/tokens';
import {
  CLASSES,
  type Classe,
  classeDe,
  contagemDeSkills,
  GRUPOS,
  rotuloDaContagem,
  tituloDe,
} from './contratos';
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
        <Typography variant="h4" component="h1" sx={{ color: 'secondary.main' }}>
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
  skills: readonly SkillResumo[];
  filtro: string;
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function ResultadoDaConsulta({
  consulta,
  skills,
  filtro,
  podeEditar,
  onAbrir,
}: ResultadoProps): ReactNode {
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
      skills={skills}
      temFiltro={filtro.trim() !== ''}
      podeEditar={podeEditar}
      onAbrir={onAbrir}
    />
  );
}

function CampoDeFiltro({ filtro, onMudar }: { filtro: string; onMudar: (texto: string) => void }) {
  return (
    <TextField
      placeholder="Filtrar por nome ou descrição"
      size="small"
      value={filtro}
      onChange={(evento) => onMudar(evento.target.value)}
      slotProps={{
        htmlInput: { 'aria-label': 'Filtrar' },
        input: {
          startAdornment: (
            <InputAdornment position="start">
              <SearchIcon sx={{ fontSize: ICONES.controle, color: 'text.muted' }} />
            </InputAdornment>
          ),
        },
      }}
      sx={{ width: LARGURA_DA_LISTA_ESTREITA, '& .MuiInputBase-root': { height: 36 } }}
    />
  );
}

export function FerramentasPage(): ReactNode {
  const consulta = useFerramentas();
  const skills = useSkills().data?.skills ?? [];
  const podeEditar = usePodeEditar('ferramentas.editar');
  const [filtro, setFiltro] = useState('');
  const [aberta, setAberta] = useState<Aberta>(undefined);
  useContextoDaTela({ tela: 'ferramentas' });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
      <CabecalhoDaPagina podeEditar={podeEditar} onNova={() => setAberta(null)} />
      <CampoDeFiltro filtro={filtro} onMudar={setFiltro} />
      <ResultadoDaConsulta
        consulta={consulta}
        skills={skills}
        filtro={filtro}
        podeEditar={podeEditar}
        onAbrir={setAberta}
      />
      {aberta !== undefined && (
        <FerramentaDrawer
          key={aberta?.name ?? 'nova'}
          ferramenta={aberta}
          skills={skills}
          podeEditar={podeEditar}
          onFechar={() => setAberta(undefined)}
        />
      )}
    </Box>
  );
}

type GruposProps = {
  ferramentas: readonly Ferramenta[];
  skills: readonly SkillResumo[];
  temFiltro: boolean;
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function Grupos({ ferramentas, skills, temFiltro, podeEditar, onAbrir }: GruposProps): ReactNode {
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
      skills={skills}
      podeEditar={podeEditar}
      onAbrir={onAbrir}
    />
  ));
}

const ESTILO_DO_ROTULO_DE_SECAO = {
  fontSize: TIPOS.overline,
  fontWeight: PESOS.semi,
  color: 'text.tertiary',
  textTransform: 'uppercase',
  letterSpacing: '.06em',
} as const;

type GrupoProps = {
  classe: Classe;
  grupo: readonly Ferramenta[];
  skills: readonly SkillResumo[];
  podeEditar: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function GrupoDeFerramentas({ classe, grupo, skills, podeEditar, onAbrir }: GrupoProps): ReactNode {
  return (
    <Box component="section" sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Typography component="h2" sx={ESTILO_DO_ROTULO_DE_SECAO}>
        {GRUPOS[classe]}
      </Typography>
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: 'repeat(3, minmax(0, 1fr))' }}>
        {grupo.map((ferramenta) => (
          <CartaoDeFerramenta
            key={ferramenta.name}
            ferramenta={ferramenta}
            quantidadeDeSkills={contagemDeSkills(skills, ferramenta.name)}
            clicavel={podeEditar}
            onAbrir={onAbrir}
          />
        ))}
      </Box>
    </Box>
  );
}

type RotuloProps = { tom: 'registra' | 'neutro'; children: ReactNode };

function RotuloDoCartao({ tom, children }: RotuloProps): ReactNode {
  return (
    <Chip
      size="small"
      label={children}
      sx={{
        borderRadius: RAIO.pilula,
        fontSize: TIPOS.overline,
        fontWeight: PESOS.medio,
        ...(tom === 'registra'
          ? { bgcolor: 'studio.registra.soft', color: 'studio.registra.onSoft' }
          : { bgcolor: 'background.neutralFill', color: 'text.secondary' }),
      }}
    />
  );
}

type ConteudoProps = { ferramenta: Ferramenta; quantidadeDeSkills: number; comDescricao: boolean };

function ConteudoDoCartao({ ferramenta, quantidadeDeSkills, comDescricao }: ConteudoProps) {
  return (
    <CardContent sx={{ display: 'flex', flexDirection: 'column', gap: 1, height: '100%' }}>
      <Typography variant="subtitle2" component="h3">
        {tituloDe(ferramenta, comDescricao)}
      </Typography>
      {comDescricao && (
        <Typography
          variant="body2"
          color="text.secondary"
          sx={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        >
          {ferramenta.description}
        </Typography>
      )}
      <Box sx={{ display: 'flex', gap: 1, mt: 0.5 }}>
        {ferramenta.write && <RotuloDoCartao tom="registra">Grava no sistema</RotuloDoCartao>}
        <RotuloDoCartao tom="neutro">{rotuloDaContagem(quantidadeDeSkills)}</RotuloDoCartao>
      </Box>
    </CardContent>
  );
}

type CartaoProps = {
  ferramenta: Ferramenta;
  quantidadeDeSkills: number;
  clicavel: boolean;
  onAbrir: (ferramenta: Ferramenta) => void;
};

function CartaoDeFerramenta({
  ferramenta,
  quantidadeDeSkills,
  clicavel,
  onAbrir,
}: CartaoProps): ReactNode {
  // Quem não edita não vê a descrição técnica da ferramenta.
  const conteudo = (
    <ConteudoDoCartao
      ferramenta={ferramenta}
      quantidadeDeSkills={quantidadeDeSkills}
      comDescricao={clicavel}
    />
  );
  return (
    <Card variant="outlined" sx={{ borderRadius: RAIO.painel }}>
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

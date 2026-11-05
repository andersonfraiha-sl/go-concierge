import { Box, Switch, TableCell, TableRow, Tooltip } from '@mui/material';
import type { ReactNode } from 'react';
import { PESOS } from '@/theme/tokens';
import { type Caso, habilitacaoDe, type Mapeamento, PADRAO } from './contratos';
import type { LinhaDeSkill } from './listaDeSkills';

export type Coluna = { id: string; nome: string };
export type CelulaAberta = { caso: Caso; coluna: Coluna };
export type Gravar = (
  caso: Caso,
  coluna: Coluna,
  habilitado: boolean,
  papeis: string[],
) => Promise<boolean>;

/** Agrupa o que toda célula da matriz precisa, para manter cada componente com poucos parâmetros. */
export type ContextoDaMatriz = {
  mapeamento: Mapeamento;
  colunas: Coluna[];
  editavelNaColuna: (coluna: Coluna) => boolean;
  gravando: boolean;
  onGravar: Gravar;
  onAbrirCelula: (aberta: CelulaAberta) => void;
};

export const ESTILO_COLUNA_FIXA = {
  position: 'sticky' as const,
  left: 0,
  bgcolor: 'background.paper',
  zIndex: 1,
  whiteSpace: 'nowrap' as const,
};

export function PontoDeDecisao(): ReactNode {
  return (
    <Tooltip title="Decisão deste condomínio">
      <Box
        role="img"
        aria-label="Decisão deste condomínio"
        sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: 'primary.main', flex: 'none' }}
      />
    </Tooltip>
  );
}

const ESTILO_DO_TITULO = {
  fontWeight: PESOS.semi,
  borderColor: 'border.subtle',
} as const;

export function LinhaDaMatriz({
  contexto,
  linha,
}: {
  contexto: ContextoDaMatriz;
  linha: LinhaDeSkill;
}): ReactNode {
  return (
    <TableRow>
      <TableCell component="th" scope="row" sx={[ESTILO_COLUNA_FIXA, ESTILO_DO_TITULO]}>
        {linha.titulo}
      </TableCell>
      {contexto.colunas.map((coluna) => (
        <Celula key={coluna.id} contexto={contexto} linha={linha} coluna={coluna} />
      ))}
    </TableRow>
  );
}

type CelulaProps = { contexto: ContextoDaMatriz; linha: LinhaDeSkill; coluna: Coluna };

const ESTILO_DO_BOTAO_DA_CELULA = {
  position: 'absolute',
  inset: 0,
  width: '100%',
  height: '100%',
  border: 0,
  padding: 0,
  background: 'none',
  cursor: 'pointer',
} as const;

const ESTILO_DO_INTERRUPTOR = {
  position: 'relative',
  zIndex: 1,
  display: 'inline-flex',
  alignItems: 'center',
  gap: 0.75,
} as const;

function Celula({ contexto, linha, coluna }: CelulaProps): ReactNode {
  return (
    <TableCell align="center" sx={{ position: 'relative', borderColor: 'border.subtle' }}>
      <Box
        component="button"
        type="button"
        aria-label={`Ver papéis, orientação e ajustes de ${linha.titulo} em ${coluna.nome}`}
        onClick={() => contexto.onAbrirCelula({ caso: linha.caso, coluna })}
        sx={ESTILO_DO_BOTAO_DA_CELULA}
      />
      <InterruptorDaCelula contexto={contexto} linha={linha} coluna={coluna} />
    </TableCell>
  );
}

function InterruptorDaCelula({ contexto, linha, coluna }: CelulaProps): ReactNode {
  const { habilitacao, propria } = habilitacaoDe(contexto.mapeamento, coluna.id, linha.caso.nome);
  const mostraPontoDeDecisao = propria && coluna.id !== PADRAO;

  return (
    <Box sx={ESTILO_DO_INTERRUPTOR}>
      <Switch
        size="small"
        checked={habilitacao.habilitado}
        disabled={!contexto.editavelNaColuna(coluna) || contexto.gravando}
        onChange={(_, ligado) =>
          void contexto.onGravar(linha.caso, coluna, ligado, habilitacao.papeis)
        }
        slotProps={{ input: { 'aria-label': `${linha.titulo} em ${coluna.nome}` } }}
      />
      {mostraPontoDeDecisao && <PontoDeDecisao />}
    </Box>
  );
}

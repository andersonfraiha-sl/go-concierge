import {
  Box,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { CONDOMINIOS, PESOS, RAIOS } from '@/theme/tokens';
import {
  type Administradora,
  type Condominio,
  type Estado,
  PADRAO,
  salvarHabilitacao,
  useEscopo,
} from './contratos';
import {
  agruparPorCategoria,
  linhasDeSkills,
  paraSentenceCase,
  ROTULO_DA_CATEGORIA,
} from './listaDeSkills';
import {
  type CelulaAberta,
  type Coluna,
  type ContextoDaMatriz,
  ESTILO_COLUNA_FIXA,
  type Gravar,
  LinhaDaMatriz,
  PontoDeDecisao,
} from './MatrizDeSkillsLinha';
import { OpcoesDeAdministradora } from './SeletorDeAdministradora';
import { SkillDoCondominioDrawer } from './SkillDoCondominioDrawer';
import { useExecutar } from './useExecutar';

type Props = { estado: Estado; titulos: ReadonlyMap<string, string>; editavel: boolean };
type Execucao = ReturnType<typeof useExecutar>;
type Filtro = ReturnType<typeof useFiltroDeAdministradora>;

/** Só a Superlógica (sem escopo) filtra por qualquer administradora; as outras ficam na própria. */
function useFiltroDeAdministradora() {
  const escopo = useEscopo();
  const [escolhido, escolher] = useState('');
  const travado = escopo !== null;
  return { valor: travado ? (escopo ?? '') : escolhido, travado, escolher };
}

const gravarCelula =
  (executar: Execucao['executar']): Gravar =>
  (caso, coluna, habilitado, papeis) =>
    executar((escopo) =>
      salvarHabilitacao(
        { condominio: coluna.id, caso: caso.nome, habilitado, papeis, limpar: false },
        escopo,
      ),
    );

function colunasDaMatriz(condominios: Condominio[], filtro: string): Coluna[] {
  const filtrados =
    filtro === '' ? condominios : condominios.filter((c) => c.administradora_id === filtro);
  return [{ id: PADRAO, nome: 'Padrão' }, ...filtrados.map((c) => ({ id: c.id, nome: c.nome }))];
}

type EntradaDoContexto = { estado: Estado; editavel: boolean; filtro: Filtro };

function useContextoDaMatriz({ estado, editavel, filtro }: EntradaDoContexto) {
  const { executar, gravando } = useExecutar();
  const [celulaAberta, setCelulaAberta] = useState<CelulaAberta | null>(null);
  const colunas = colunasDaMatriz(estado.mapeamento.condominios, filtro.valor);
  const ehPadraoTravado = (coluna: Coluna) => coluna.id === PADRAO && filtro.travado;

  const contexto: ContextoDaMatriz = {
    mapeamento: estado.mapeamento,
    colunas,
    editavelNaColuna: (coluna) => editavel && !ehPadraoTravado(coluna),
    gravando,
    onGravar: gravarCelula(executar),
    onAbrirCelula: setCelulaAberta,
  };
  return { contexto, celulaAberta, fechar: () => setCelulaAberta(null) };
}

export function MatrizDeSkills({ estado, titulos, editavel }: Props): ReactNode {
  const filtro = useFiltroDeAdministradora();
  const { contexto, celulaAberta, fechar } = useContextoDaMatriz({ estado, editavel, filtro });
  const grupos = agruparPorCategoria(linhasDeSkills(estado, PADRAO, titulos));

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <BarraDaMatriz administradoras={estado.administradoras} filtro={filtro} />
      <TabelaDaMatriz contexto={contexto} grupos={grupos} />
      <SkillDaCelula
        celula={celulaAberta}
        estado={estado}
        titulos={titulos}
        contexto={contexto}
        onFechar={fechar}
      />
    </Box>
  );
}

function BarraDaMatriz(props: { administradoras: Administradora[]; filtro: Filtro }): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2.5 }}>
      <LegendaDaMatriz />
      <Box sx={{ flex: 1 }} />
      <FiltroDeAdministradora
        administradoras={props.administradoras}
        valor={props.filtro.valor}
        travado={props.filtro.travado}
        onMudar={props.filtro.escolher}
      />
    </Box>
  );
}

type TabelaProps = { contexto: ContextoDaMatriz; grupos: ReturnType<typeof agruparPorCategoria> };

function TabelaDaMatriz({ contexto, grupos }: TabelaProps): ReactNode {
  return (
    <Paper variant="outlined" sx={{ borderRadius: `${RAIOS.cartao}px`, overflow: 'hidden' }}>
      <Box sx={{ overflow: 'auto' }}>
        <Table size="small">
          <CabecalhoDaMatriz colunas={contexto.colunas} />
          <TableBody>
            {grupos.map((grupo) => (
              <GrupoDaMatriz key={grupo.categoria} grupo={grupo} contexto={contexto} />
            ))}
          </TableBody>
        </Table>
      </Box>
    </Paper>
  );
}

const ESTILO_DO_CABECALHO = {
  fontWeight: PESOS.semi,
  bgcolor: 'background.paper',
  color: 'text.strong',
  py: 1.25,
  px: 3,
} as const;

function CabecalhoDaMatriz({ colunas }: { colunas: Coluna[] }): ReactNode {
  return (
    <TableHead>
      <TableRow>
        <TableCell component="th" scope="col" sx={[ESTILO_COLUNA_FIXA, ESTILO_DO_CABECALHO]}>
          Skill
        </TableCell>
        {colunas.map((coluna) => (
          <TableCell key={coluna.id} scope="col" align="center" sx={ESTILO_DO_CABECALHO}>
            {coluna.nome}
          </TableCell>
        ))}
      </TableRow>
    </TableHead>
  );
}

type Grupo = ReturnType<typeof agruparPorCategoria>[number];

const ESTILO_DO_GRUPO = {
  bgcolor: 'background.subtle',
  typography: 'overline',
  fontWeight: PESOS.semi,
  color: 'text.tertiary',
  textTransform: 'uppercase',
  borderBottom: 1,
  borderColor: 'border.subtle',
  py: 1.25,
  px: 3,
} as const;

function GrupoDaMatriz({ grupo, contexto }: { grupo: Grupo; contexto: ContextoDaMatriz }) {
  return (
    <>
      <TableRow>
        <TableCell colSpan={contexto.colunas.length + 1} sx={ESTILO_DO_GRUPO}>
          {ROTULO_DA_CATEGORIA[grupo.categoria]}
        </TableCell>
      </TableRow>
      {grupo.linhas.map((linha) => (
        <LinhaDaMatriz key={linha.caso.nome} contexto={contexto} linha={linha} />
      ))}
    </>
  );
}

type SkillDaCelulaProps = {
  celula: CelulaAberta | null;
  estado: Estado;
  titulos: ReadonlyMap<string, string>;
  contexto: ContextoDaMatriz;
  onFechar: () => void;
};

function SkillDaCelula({ celula, estado, titulos, contexto, onFechar }: SkillDaCelulaProps) {
  if (celula === null) return null;
  return (
    <SkillDoCondominioDrawer
      estado={estado}
      condominio={celula.coluna.id}
      caso={celula.caso}
      titulo={paraSentenceCase(titulos.get(celula.caso.nome) ?? celula.caso.titulo)}
      editavel={contexto.editavelNaColuna(celula.coluna)}
      onFechar={onFechar}
    />
  );
}

type FiltroProps = {
  administradoras: Administradora[];
  valor: string;
  travado: boolean;
  onMudar: (id: string) => void;
};

function FiltroDeAdministradora({ administradoras, valor, travado, onMudar }: FiltroProps) {
  return (
    <TextField
      select
      size="small"
      value={valor}
      disabled={travado}
      onChange={(e) => onMudar(e.target.value)}
      sx={{ width: CONDOMINIOS.larguraDoFiltro, bgcolor: 'background.paper' }}
      slotProps={{ select: { native: true, 'aria-label': 'Administradora' } }}
    >
      <option value="">Todas as administradoras</option>
      <OpcoesDeAdministradora administradoras={administradoras} />
    </TextField>
  );
}

function LegendaDaMatriz(): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 2.5, color: 'text.tertiary' }}>
      <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.75 }}>
        <PontoDeDecisao />
        <Typography variant="caption">decisão deste condomínio</Typography>
      </Box>
      <Typography variant="caption">sem ponto: segue o Padrão</Typography>
      <Typography variant="caption">
        clique na célula para ver papéis, orientação e ajustes
      </Typography>
    </Box>
  );
}

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
import { Fragment, type ReactNode, useState } from 'react';
import {
  type Administradora,
  type Estado,
  PADRAO,
  salvarHabilitacao,
  useEscopo,
} from './contratos';
import { agruparPorCategoria, linhasDeSkills, ROTULO_DA_CATEGORIA } from './listaDeSkills';
import {
  type CelulaAberta,
  type Coluna,
  type ContextoDaMatriz,
  ESTILO_COLUNA_FIXA,
  LinhaDaMatriz,
  PontoDeDecisao,
} from './MatrizDeSkillsLinha';
import { SkillDoCondominioDrawer } from './SkillDoCondominioDrawer';
import { useExecutar } from './useExecutar';

type Props = { estado: Estado; titulos: ReadonlyMap<string, string>; editavel: boolean };

export function MatrizDeSkills({ estado, titulos, editavel }: Props): ReactNode {
  const escopo = useEscopo();
  const { executar, gravando } = useExecutar();
  const [celulaAberta, setCelulaAberta] = useState<CelulaAberta | null>(null);
  const [filtroEscolhido, setFiltroEscolhido] = useState('');

  // Só a Superlógica (sem escopo) muda o Padrão e filtra por qualquer administradora;
  // quem pertence a uma administradora fica travado na própria, nos dois casos.
  const travadoNaPropriaAdministradora = escopo !== null;
  const filtro = travadoNaPropriaAdministradora ? (escopo ?? '') : filtroEscolhido;
  const condominiosFiltrados =
    filtro === ''
      ? estado.mapeamento.condominios
      : estado.mapeamento.condominios.filter((c) => c.administradora_id === filtro);

  const colunas: Coluna[] = [
    { id: PADRAO, nome: 'Padrão' },
    ...condominiosFiltrados.map((c) => ({ id: c.id, nome: c.nome })),
  ];
  const grupos = agruparPorCategoria(linhasDeSkills(estado, PADRAO, titulos));

  const contexto: ContextoDaMatriz = {
    mapeamento: estado.mapeamento,
    colunas,
    editavelNaColuna: (coluna) =>
      editavel && !(coluna.id === PADRAO && travadoNaPropriaAdministradora),
    gravando,
    onGravar: (caso, coluna, habilitado, papeis) =>
      executar((e) =>
        salvarHabilitacao(
          { condominio: coluna.id, caso: caso.nome, habilitado, papeis, limpar: false },
          e,
        ),
      ),
    onAbrirCelula: setCelulaAberta,
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2.5 }}>
        <LegendaDaMatriz />
        <Box sx={{ flex: 1 }} />
        <FiltroDeAdministradora
          administradoras={estado.administradoras}
          valor={filtro}
          travado={travadoNaPropriaAdministradora}
          onMudar={setFiltroEscolhido}
        />
      </Box>
      <Paper variant="outlined" sx={{ overflow: 'auto' }}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell component="th" scope="col" sx={ESTILO_COLUNA_FIXA}>
                Skill
              </TableCell>
              {colunas.map((coluna) => (
                <TableCell key={coluna.id} scope="col" align="center">
                  {coluna.nome}
                </TableCell>
              ))}
            </TableRow>
          </TableHead>
          <TableBody>
            {grupos.map((grupo) => (
              <Fragment key={grupo.categoria}>
                <TableRow>
                  <TableCell
                    colSpan={colunas.length + 1}
                    sx={{
                      bgcolor: 'background.default',
                      typography: 'overline',
                      color: 'text.tertiary',
                    }}
                  >
                    {ROTULO_DA_CATEGORIA[grupo.categoria]}
                  </TableCell>
                </TableRow>
                {grupo.linhas.map((linha) => (
                  <LinhaDaMatriz key={linha.caso.nome} contexto={contexto} linha={linha} />
                ))}
              </Fragment>
            ))}
          </TableBody>
        </Table>
      </Paper>
      {celulaAberta && (
        <SkillDoCondominioDrawer
          estado={estado}
          condominio={celulaAberta.coluna.id}
          caso={celulaAberta.caso}
          titulo={titulos.get(celulaAberta.caso.nome) ?? celulaAberta.caso.titulo}
          editavel={contexto.editavelNaColuna(celulaAberta.coluna)}
          onFechar={() => setCelulaAberta(null)}
        />
      )}
    </Box>
  );
}

function FiltroDeAdministradora({
  administradoras,
  valor,
  travado,
  onMudar,
}: {
  administradoras: Administradora[];
  valor: string;
  travado: boolean;
  onMudar: (id: string) => void;
}): ReactNode {
  return (
    <TextField
      select
      size="small"
      label="Administradora"
      value={valor}
      disabled={travado}
      onChange={(e) => onMudar(e.target.value)}
      sx={{ width: 240 }}
      slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
    >
      <option value="">Todas as administradoras</option>
      {administradoras.map((a) => (
        <option key={a.id} value={a.id}>
          {a.nome}
        </option>
      ))}
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

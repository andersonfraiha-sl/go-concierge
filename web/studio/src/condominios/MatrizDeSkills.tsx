import {
  Box,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  Typography,
} from '@mui/material';
import { Fragment, type ReactNode, useState } from 'react';
import { type Estado, PADRAO, salvarHabilitacao, useEscopo } from './contratos';
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

  const colunas: Coluna[] = [
    { id: PADRAO, nome: 'Padrão' },
    ...estado.mapeamento.condominios.map((c) => ({ id: c.id, nome: c.nome })),
  ];
  const grupos = agruparPorCategoria(linhasDeSkills(estado, PADRAO, titulos));
  // O padrão vale para todas as administradoras: só a Superlógica (sem escopo) o altera.
  const padraoTravado = escopo !== null;

  const contexto: ContextoDaMatriz = {
    mapeamento: estado.mapeamento,
    colunas,
    editavelNaColuna: (coluna) => editavel && !(coluna.id === PADRAO && padraoTravado),
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
      <LegendaDaMatriz />
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

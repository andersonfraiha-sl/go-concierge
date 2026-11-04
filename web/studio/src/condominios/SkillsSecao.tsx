import SearchOutlined from '@mui/icons-material/SearchOutlined';
import { Box, InputAdornment, Switch, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { type Estado, PADRAO, salvarHabilitacao } from './contratos';
import { LinhaDaSkill } from './LinhaDaSkill';
import {
  agruparPorCategoria,
  contarLigadas,
  filtrarLinhas,
  type LinhaDeSkill,
  linhasDeSkills,
  ROTULO_DA_CATEGORIA,
} from './listaDeSkills';
import { SkillDoCondominioDrawer } from './SkillDoCondominioDrawer';
import { useExecutar } from './useExecutar';

type Props = {
  estado: Estado;
  condominio: string;
  titulos: ReadonlyMap<string, string>;
  editavel: boolean;
};

export function SkillsSecao({ estado, condominio, titulos, editavel }: Props): ReactNode {
  const { executar, gravando } = useExecutar();
  const [busca, setBusca] = useState('');
  const [soQueMuda, setSoQueMuda] = useState(false);
  const [skillAberta, setSkillAberta] = useState<LinhaDeSkill | null>(null);
  const ehPadrao = condominio === PADRAO;

  const linhas = linhasDeSkills(estado, condominio, titulos);
  const { ligadas, total } = contarLigadas(linhas);
  const grupos = agruparPorCategoria(filtrarLinhas(linhas, { busca, soQueMuda }));

  const gravar = (linha: LinhaDeSkill, habilitado: boolean): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao(
        {
          condominio,
          caso: linha.caso.nome,
          habilitado,
          papeis: linha.habilitacao.papeis,
          limpar: false,
        },
        escopo,
      ),
    );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: 1.5, flexWrap: 'wrap' }}>
        <Box sx={{ flex: '1 1 260px' }}>
          <Typography variant="h5" component="h2">
            O que o concierge faz aqui
          </Typography>
          <Typography variant="body2" color="text.tertiary">
            {`${ligadas} de ${total} ligadas`}
          </Typography>
        </Box>
        <TextField
          size="small"
          placeholder="Buscar…"
          value={busca}
          onChange={(evento) => setBusca(evento.target.value)}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchOutlined fontSize="small" />
                </InputAdornment>
              ),
            },
            htmlInput: { 'aria-label': 'Buscar skill' },
          }}
        />
        {!ehPadrao && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Switch
              size="small"
              checked={soQueMuda}
              onChange={(_, ligado) => setSoQueMuda(ligado)}
              slotProps={{ input: { 'aria-label': 'Só o que muda do Padrão' } }}
            />
            <Typography variant="body2">Só o que muda do Padrão</Typography>
          </Box>
        )}
      </Box>
      <Box sx={{ border: 1, borderColor: 'border.subtle', borderRadius: 13, overflow: 'hidden' }}>
        {grupos.length === 0 && (
          <Typography color="text.secondary" sx={{ p: 2 }}>
            Nenhuma skill encontrada.
          </Typography>
        )}
        {grupos.map((grupo) => (
          <Box key={grupo.categoria}>
            <Typography
              variant="overline"
              sx={{
                display: 'block',
                px: 2.5,
                py: 1.25,
                bgcolor: 'background.subtle',
                color: 'text.tertiary',
                fontWeight: 600,
              }}
            >
              {ROTULO_DA_CATEGORIA[grupo.categoria]}
            </Typography>
            {grupo.linhas.map((linha) => (
              <LinhaDaSkill
                key={linha.caso.nome}
                linha={linha}
                ehPadrao={ehPadrao}
                editavel={editavel}
                gravando={gravando}
                onAbrir={() => setSkillAberta(linha)}
                onMudarHabilitado={(habilitado) => void gravar(linha, habilitado)}
              />
            ))}
          </Box>
        ))}
      </Box>
      {skillAberta && (
        <SkillDoCondominioDrawer
          estado={estado}
          condominio={condominio}
          caso={skillAberta.caso}
          titulo={skillAberta.titulo}
          editavel={editavel}
          onFechar={() => setSkillAberta(null)}
        />
      )}
    </Box>
  );
}

import ChevronRightOutlined from '@mui/icons-material/ChevronRightOutlined';
import { Box, ListItemButton, Switch, Tooltip, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { iconeDaSkill } from '@/skills/icones';
import { type LinhaDeSkill, rotuloDePapeis } from './listaDeSkills';

type Props = {
  linha: LinhaDeSkill;
  ehPadrao: boolean;
  editavel: boolean;
  gravando: boolean;
  onAbrir: () => void;
  onMudarHabilitado: (habilitado: boolean) => void;
};

/** Linha densa de ~52 px: ícone, título, resumo, chips de exceção, atalho e switch. */
export function LinhaDaSkill({
  linha,
  ehPadrao,
  editavel,
  gravando,
  onAbrir,
  onMudarHabilitado,
}: Props): ReactNode {
  const Icone = iconeDaSkill(linha.caso.icone);
  const rotuloPapeis = rotuloDePapeis(linha.habilitacao.papeis);
  const mostraDecisaoPropria = linha.propria && !ehPadrao;

  return (
    <ListItemButton
      onClick={onAbrir}
      sx={{ minHeight: 52, gap: 1.5, px: 2.5, borderTop: 1, borderColor: 'border.subtle' }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          flex: 1,
          minWidth: 0,
          opacity: linha.habilitacao.habilitado ? 1 : 0.6,
        }}
      >
        <Box
          sx={{
            width: 30,
            height: 30,
            borderRadius: 1,
            bgcolor: 'background.tint',
            color: 'primary.main',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flex: 'none',
          }}
        >
          <Icone sx={{ fontSize: 18 }} />
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1.25, flex: 1, minWidth: 0 }}>
          <Typography sx={{ fontWeight: 600, whiteSpace: 'nowrap' }}>{linha.titulo}</Typography>
          <Tooltip title={linha.caso.resumo}>
            <Typography variant="body2" color="text.muted" noWrap sx={{ flex: 1, minWidth: 0 }}>
              {linha.caso.resumo}
            </Typography>
          </Tooltip>
        </Box>
        {rotuloPapeis !== null && (
          <Typography
            variant="caption"
            sx={{
              px: 1,
              py: 0.25,
              borderRadius: 99,
              bgcolor: 'background.tint',
              color: 'primary.main',
              whiteSpace: 'nowrap',
            }}
          >
            {rotuloPapeis}
          </Typography>
        )}
        {mostraDecisaoPropria && (
          <Typography
            variant="caption"
            sx={{
              px: 1,
              py: 0.25,
              borderRadius: 99,
              bgcolor: 'warning.background',
              color: 'warning.text',
              whiteSpace: 'nowrap',
            }}
          >
            deste condomínio
          </Typography>
        )}
      </Box>
      <ChevronRightOutlined fontSize="small" sx={{ color: 'text.muted', flex: 'none' }} />
      <Switch
        checked={linha.habilitacao.habilitado}
        disabled={!editavel || gravando}
        onClick={(evento) => evento.stopPropagation()}
        onChange={(_, ligado) => onMudarHabilitado(ligado)}
        slotProps={{ input: { 'aria-label': `${linha.titulo} neste condomínio` } }}
      />
    </ListItemButton>
  );
}

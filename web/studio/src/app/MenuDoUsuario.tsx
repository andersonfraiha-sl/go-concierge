import { Box, ButtonBase, Popover, type SxProps, type Theme, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { DevRoleSwitch } from '@/acesso/DevRoleSwitch';
import { type Eu, useEu } from '@/acesso/eu';
import { PESOS, RAIO, SHELL, TIPOS } from '@/theme/tokens';

const ROTULO_DO_PAPEL: Record<string, string> = {
  admin_superlogica: 'Administrador · Superlógica',
  construtor_superlogica: 'Construtor · Superlógica',
  gestor_administradora: 'Gestor · Administradora',
};

/** Papel sem rótulo conhecido mostra o próprio valor. */
export const rotuloDoPapel = (papel: string): string => ROTULO_DO_PAPEL[papel] ?? papel;

/** Iniciais das duas primeiras palavras do nome, ignorando o que vem entre parênteses. */
export function iniciaisDe(nome: string): string {
  const palavras = nome
    .replace(/\(.*?\)/g, '')
    .trim()
    .split(/\s+/);
  return palavras
    .slice(0, 2)
    .map((palavra) => palavra.charAt(0).toUpperCase())
    .join('');
}

const ESTILO_DO_AVATAR: SxProps<Theme> = {
  width: SHELL.avatar,
  height: SHELL.avatar,
  flexShrink: 0,
  borderRadius: RAIO.pilula,
  bgcolor: 'background.tint',
  color: 'primary.main',
  fontSize: TIPOS.body2,
  fontWeight: PESOS.semi,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

function Identidade({ eu }: { eu: Eu }): ReactNode {
  return (
    <>
      <Box sx={ESTILO_DO_AVATAR}>{iniciaisDe(eu.nome)}</Box>
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          lineHeight: SHELL.alturaDaLinha,
          minWidth: 0,
        }}
      >
        <Typography
          component="span"
          sx={{ fontSize: TIPOS.body2, fontWeight: PESOS.semi, lineHeight: 'inherit' }}
        >
          {eu.nome}
        </Typography>
        <Typography
          component="span"
          sx={{ fontSize: TIPOS.overline, color: 'text.tertiary', lineHeight: 'inherit' }}
        >
          {rotuloDoPapel(eu.papel)}
        </Typography>
      </Box>
    </>
  );
}

/** Avatar com nome e papel; em dev, abre o "Ver como" para trocar de usuário. */
export function MenuDoUsuario(): ReactNode {
  const { data: eu } = useEu();
  if (!eu) return null;
  if (!eu.dev) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, textAlign: 'left' }}>
        <Identidade eu={eu} />
      </Box>
    );
  }
  return <IdentidadeComVerComo eu={eu} />;
}

function IdentidadeComVerComo({ eu }: { eu: Eu }): ReactNode {
  const [ancora, setAncora] = useState<HTMLElement | null>(null);
  return (
    <>
      <ButtonBase
        aria-label="Conta e Ver como"
        aria-haspopup="dialog"
        aria-expanded={ancora !== null}
        onClick={(evento) => setAncora(evento.currentTarget)}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          textAlign: 'left',
          borderRadius: RAIO.menu,
        }}
      >
        <Identidade eu={eu} />
      </ButtonBase>
      <Popover
        open={ancora !== null}
        anchorEl={ancora}
        onClose={() => setAncora(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{ paper: { sx: { p: 2, mt: 1, border: 1, borderColor: 'border.default' } } }}
      >
        <DevRoleSwitch usuarioAtual={eu.id} />
      </Popover>
    </>
  );
}

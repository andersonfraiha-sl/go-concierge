import { SvgIcon, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';
import { ICONES, SHELL } from '@/theme/tokens';

// Ícones de linha do desenho aprovado (viewBox 24, traço arredondado), fiéis ao board.
const TRACOS = {
  concierge: <path d="M4 18h16M6 18a6 6 0 0 1 12 0M12 9V7" />,
  skills: <path d="M4 7h16M4 12h16M4 17h10" />,
  ferramentas: <path d="M14 7l3-3 3 3-3 3M17 4v9a4 4 0 0 1-4 4H4" />,
  condominios: <path d="M4 21V5l8-2v18M12 9l8 2v10M8 9h1M8 13h1M8 17h1M16 14h1M16 18h1" />,
  avancado: (
    <>
      <path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0" />
      <circle cx="16" cy="6" r="2" />
      <circle cx="10" cy="12" r="2" />
      <circle cx="18" cy="18" r="2" />
    </>
  ),
  atendente: <path d="M4 5h16v11H8l-4 4z" />,
  linkExterno: <path d="M14 4h6v6M20 4l-9 9M18 14v6H4V6h6" />,
  busca: (
    <>
      <circle cx="11" cy="11" r="7" />
      <path d="m20 20-3.5-3.5" />
    </>
  ),
  copiloto: (
    <>
      <path d="M12 3l1.8 4.6L18 9.4l-4.2 1.8L12 16l-1.8-4.8L6 9.4l4.2-1.8z" />
      <path d="M19 15l.8 2 2 .8-2 .8-.8 2-.8-2-2-.8 2-.8z" />
    </>
  ),
  voltar: <path d="M15 6l-6 6 6 6" />,
  recolher: <path d="M15 6l-6 6 6 6" />,
  expandir: <path d="M9 6l6 6-6 6" />,
} as const satisfies Record<string, ReactNode>;

export type NomeDoIcone = keyof typeof TRACOS;

type Props = { nome: NomeDoIcone; tamanho?: number; traco?: number; sx?: SxProps<Theme> };

export function IconeDeLinha({
  nome,
  tamanho = ICONES.grande,
  traco = SHELL.traco,
  sx,
}: Props): ReactNode {
  return (
    <SvgIcon
      aria-hidden
      sx={[
        {
          fontSize: tamanho,
          fill: 'none',
          stroke: 'currentColor',
          strokeWidth: traco,
          strokeLinecap: 'round',
        },
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
    >
      {TRACOS[nome]}
    </SvgIcon>
  );
}

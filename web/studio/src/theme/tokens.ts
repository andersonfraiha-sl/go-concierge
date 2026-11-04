// Identidade visual do desenho aprovado em docs/superpowers/design/studio-fase2.
const FONTE_DE_TITULO = '"Paralucent", "Plus Jakarta Sans", system-ui, sans-serif';
const FONTE_DE_CORPO = '"Argumentum", "Inter", system-ui, sans-serif';

export const FONTE_MONO = '"JetBrains Mono", ui-monospace, monospace';

// Proporção áurea: 8 / 13 / 21 / 34 / 55 px (theme.spacing(1..5)).
export const ESPACOS = [0, 8, 13, 21, 34, 55] as const;

// Escala de tipos do desenho aprovado, em px.
export const TIPOS = {
  overline: 11,
  caption: 12,
  body2: 13,
  body1: 14,
  subtitle1: 15,
  h6: 17,
  h5: 21,
  h4: 26,
} as const;

export const RAIOS = {
  campo: 8,
  chip: 9,
  menu: 10,
  painel: 12,
  cartao: 13,
} as const;

export const SOMBRA_DO_BLOCO_EM_EDICAO = '0 8px 21px rgba(16,52,242,.12)';

const titulo = { fontFamily: FONTE_DE_TITULO, fontWeight: 700 } as const;

export const tokens = {
  color: {
    primary: {
      main: '#1034F2',
      dark: '#0A24B8',
      contrastText: '#ffffff',
      outlinedBorder: '#1034f280',
    },
    navy: '#071950',
    text: {
      primary: '#22252B',
      strong: '#323B49',
      secondary: '#536379',
      tertiary: '#637792',
      muted: '#7789A3',
    },
    border: {
      default: '#E4E7EC',
      strong: '#DBDFE5',
      subtle: '#EDEFF2',
    },
    background: {
      default: '#F5F6F8',
      paper: '#FFFFFF',
      subtle: '#F8F9FB',
      tint: '#EEF1FE',
      neutralFill: '#F1F3F6',
    },
    success: { main: '#1B993B', text: '#1B7A33', background: '#E8F5EC' },
    warning: { text: '#8A5A00', background: '#FFF6DF' },
    danger: { main: '#D61D1E' },
    realceDoCopiloto: '#FFF7CC',
    conector: '#C1CAD6',
    scrollbar: { thumb: '#c1c1c1', thumbHover: '#a8a8a8' },
    studio: {
      category: {
        start: '#2E7D32',
        send: '#1034f2',
        ask: '#7B1FA2',
        logic: '#EF6C00',
        integrate: '#00838F',
        ai: '#C2185B',
        end: '#455A64',
      },
      canvasBg: '#F7F8FA',
      canvasDot: '#D0D4DB',
      nodeBg: '#ffffff',
      nodeBorder: '#DBDFE5',
      nodeSelected: '#1034f2',
      issueError: '#D61D1E',
      issueWarning: '#ef6c00',
    },
  },
  typography: {
    fontFamily: FONTE_DE_CORPO,
    fontWeightRegular: 400,
    fontWeightMedium: 500,
    fontWeightBold: 600,
    h1: { ...titulo, fontSize: `${TIPOS.h4}px` },
    h2: { ...titulo, fontSize: `${TIPOS.h4}px` },
    h3: { ...titulo, fontSize: `${TIPOS.h4}px` },
    h4: { ...titulo, fontSize: `${TIPOS.h4}px` },
    h5: { ...titulo, fontSize: `${TIPOS.h5}px` },
    h6: { ...titulo, fontSize: `${TIPOS.h6}px` },
    subtitle1: { fontSize: `${TIPOS.subtitle1}px`, fontWeight: 600 },
    body1: { fontSize: `${TIPOS.body1}px` },
    body2: { fontSize: `${TIPOS.body2}px` },
    caption: { fontSize: `${TIPOS.caption}px` },
    overline: { fontSize: `${TIPOS.overline}px`, letterSpacing: '.06em' },
  },
  shape: { borderRadius: RAIOS.campo },
  layout: {
    sidebarWidth: 236,
    sidebarCollapsedWidth: 56,
    navbarHeight: 58,
    painelLateral: 400,
    copiloto: 380,
  },
} as const;

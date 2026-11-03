// Identidade visual da Superlógica, a mesma de web/okf/styles.css (Console).
const FONTE_DE_TITULO = '"Paralucent", "Plus Jakarta Sans", system-ui, sans-serif';
const titulo = { fontFamily: FONTE_DE_TITULO } as const;

export const tokens = {
  color: {
    primary: { main: '#1034f2', contrastText: '#ffffff', outlinedBorder: '#1034f280' },
    navy: '#071950',
    text: { primary: '#22252B', secondary: '#637792' },
    background: { default: '#F5F6F8', paper: '#ffffff' },
    divider: '#DBDFE5',
    elevation: { outlined: '#DBDFE5' },
    danger: { main: '#F83333' },
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
      issueError: '#F83333',
      issueWarning: '#ef6c00',
    },
  },
  typography: {
    fontFamily: '"Argumentum", "Inter", system-ui, sans-serif',
    fontWeightRegular: 400,
    fontWeightMedium: 500,
    fontWeightBold: 600,
    h1: titulo,
    h2: titulo,
    h3: titulo,
    h4: titulo,
    h5: titulo,
    h6: titulo,
  },
  shape: { borderRadius: 8 },
  layout: { sidebarWidth: 238, sidebarCollapsedWidth: 56, navbarHeight: 60 },
} as const;

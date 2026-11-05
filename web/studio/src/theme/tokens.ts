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
  marca: 16,
  h5: 21,
  tituloEditavel: 18,
  h4: 26,
} as const;

export const RAIOS = {
  canto: 4,
  campo: 8,
  chip: 9,
  menu: 10,
  painel: 12,
  cartao: 13,
} as const;

// Para `sx`: número em `borderRadius` multiplica `shape.borderRadius`, então os raios vão como texto.
export const RAIO = {
  reto: '0',
  canto: `${RAIOS.canto}px`,
  campo: `${RAIOS.campo}px`,
  chip: `${RAIOS.chip}px`,
  menu: `${RAIOS.menu}px`,
  painel: `${RAIOS.painel}px`,
  cartao: `${RAIOS.cartao}px`,
  pilula: '999px',
} as const;

export const PESOS = { regular: 400, medio: 500, semi: 600, negrito: 700 } as const;

// Tamanhos de ícone, em px.
export const ICONES = {
  minimo: 13,
  pequeno: 14,
  controle: 15,
  medio: 16,
  entidade: 17,
  grande: 18,
  destaque: 20,
} as const;

/** Moldura do Studio (barra do topo, menu lateral, cabeçalho de detalhe, abas), em px. */
export const SHELL = {
  alturaDoControle: 34,
  alturaDoBotaoDoDetalhe: 36,
  paddingDoBotaoDeTeste: 16,
  logo: 34,
  avatar: 32,
  divisorDaMarca: 21,
  larguraDaBusca: 260,
  alturaDoItemDoMenu: 40,
  larguraMaximaDaFicha: 980,
  pontoDeSalvo: 7,
  gapDasAbas: 26,
  paddingDaAba: 13,
  indicadorDaAba: 2,
  /** Traço dos ícones de linha do desenho (o de voltar é mais grosso). */
  traco: 2,
  tracoDoVoltar: 2.2,
  alturaDaLinha: 1.2,
  alturaDaLinhaDoTitulo: 1.25,
} as const;

/** Alinhamento vertical da primeira linha, compartilhado pelo textarea e pela coluna de números. */
export const PADDING_TOPO_DO_EDITOR = '16.5px';

export const LARGURA_MINIMA_DO_CARTAO = 260;

// Larguras de gaveta lateral: cedem em tela estreita.
export const GAVETA = {
  padrao: 'min(420px, 100vw)',
  larga: 'min(560px, 100vw)',
} as const;

export const SOMBRA_DO_BLOCO_EM_EDICAO = '0 8px 21px rgba(16,52,242,.12)';

// concierge
export const CONCIERGE = {
  /** H1 "Concierge" do desenho: 26px com leve aperto de letras. */
  letterSpacingDoTitulo: '-0.26px',
  /** Base de texto dos cartões da página (board usa 16px; o resto do Studio usa 14px). */
  fonteDoCartao: 16,
  /** Pílula de tom/visibilidade: padding vertical do desenho (o horizontal já é ESPACOS[2]). */
  paddingYDaPilula: 6,
  /** Altura fixa da pílula de tom e dos botões de ação dos cartões (board: 30px / 34px). */
  alturaDaPilula: 30,
  /** Bolinha de "N mudança(s) esperando você", ainda sem proposta real para testar ao vivo. */
  pontoDePendencia: '#FFC02A',
} as const;

/** Anel interno que imita borda sem mexer no layout. */
export const anel = (cor: string, largura = 1): string => `inset 0 0 0 ${largura}px ${cor}`;

/** Interruptor no estilo iOS: trilho em pílula, pino branco que desliza. */
export const INTERRUPTOR = {
  trilhoDesligado: '#C3CAD4',
  sombraDoPino: '0 1px 3px rgba(7,25,80,.22), 0 1px 1px rgba(7,25,80,.08)',
  duracaoMs: 200,
} as const;

/** Caixa de texto: fundo branco, raio de 10 px, anel de 2 px no foco. */
export const CAMPO = {
  raio: 10,
  paddingX: 14,
  paddingY: 12,
  larguraDoAnelNoFoco: 2,
  bordaEmRepouso: '#DBDFE5',
  bordaNoHover: '#B7BFCB',
} as const;

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
      faint: '#A7B1C2',
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
    registra: { background: '#FFF3E6', text: '#C25E00' },
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

// skill
/** Tokens novos da fase 2 (área Skill: lista, Fluxo, Copiloto). */
export const TOKENS_DA_SKILL = {
  bordaDeAviso: '#F5DDAF',
  textoMuitoSuave: '#8A93A6',
  erroFundoSuave: '#FDE8E8',
  pendenciaFundo: '#FFFDF5',
} as const;

// condominios
/** Medidas específicas das telas de Condomínios (lista, matriz e ficha), do desenho aprovado. */
export const CONDOMINIOS = {
  larguraDaBusca: 300,
  larguraDoFiltro: 240,
  larguraDaColunaDeAdministradora: 239,
  larguraMaximaDoPainel: 400,
  larguraDoCampoNumerico: 70,
  letterSpacingDoTitulo: '-0.01em',
  // Mais claro que background.tint (fundo do badge "Base"), pra pílula não sumir na linha Padrão.
  fundoDaLinhaPadrao: '#F7F9FF',
} as const;

// ferramentas
/** Largura fixa da gaveta de edição de ferramenta (desenho aprovado: 460px). */
export const GAVETA_DA_FERRAMENTA = 'min(460px, 100vw)';
/** Largura e altura de listas/campos estreitos (filtro de ferramentas, árvore do Avançado). */
export const LARGURA_DA_LISTA_ESTREITA = 320;
export const ALTURA_DO_CAMPO_ESTREITO = 36;
/** Texto do rótulo "Grava no sistema" dentro do destaque laranja do drawer da ferramenta. */
export const REGISTRA_FORTE = '#8A4300';
/** Cor da chave num par chave/valor do YAML realçado (Avançado). */
export const COR_DA_CHAVE_YAML = '#7B1FA2';
/** Botão perigoso contornado (Remover/Excluir): borda e texto mais suaves que `danger.main`. */
export const PERIGO_SUAVE = { borda: '#F3C6C6', texto: '#B3191A' } as const;
/** Raio do item de arquivo selecionado/não selecionado na árvore do Avançado. */
export const RAIO_DO_ITEM_DA_ARVORE = '7px';
/** Fundo do bloco de código do editor de arquivo (Avançado), um tom abaixo do branco. */
export const FUNDO_DO_EDITOR = '#FBFCFD';
/** Lista e editor do Avançado vão até o pé da janela (topo, título, abas e seletor somam ~380 px). */
export const ALTURA_DA_AREA_DE_ARQUIVOS = 'max(480px, calc(100vh - 380px))';

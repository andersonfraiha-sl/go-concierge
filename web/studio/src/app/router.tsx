import { lazy, type ReactNode, Suspense } from 'react';
import { createBrowserRouter, Outlet, type RouteObject } from 'react-router';
import { CopilotoProvider } from '@/copiloto/CopilotoProvider';
import { LayoutDeDetalhe } from './detalhe/LayoutDeDetalhe';
// Nome do arquivo difere de rotaInicial.ts por mais que o caso da letra:
// Windows/macOS têm sistema de arquivos insensível a maiúsculas e os dois
// nomes colidiriam, fazendo o TypeScript resolver o import errado.
import { ErroDaRaiz, LimiteDeRota } from './erro/LimiteDeErro';
import { RotaInicial } from './RotaInicialRedirect';
import { StudioLayout } from './StudioLayout';

// Uma tela por chunk: quem abre o Studio baixa só a tela que está vendo.
const ConciergePage = lazy(() =>
  import('@/concierge/ConciergePage').then((m) => ({ default: m.ConciergePage })),
);
const SkillsPage = lazy(() =>
  import('@/skills/SkillsPage').then((m) => ({ default: m.SkillsPage })),
);
const SkillEditorPage = lazy(() =>
  import('@/skills/SkillEditorPage').then((m) => ({ default: m.SkillEditorPage })),
);
const EditorDaCamadaPage = lazy(() =>
  import('@/camadas/EditorDaCamadaPage').then((m) => ({ default: m.EditorDaCamadaPage })),
);
const ComparacaoPage = lazy(() =>
  import('@/camadas/ComparacaoPage').then((m) => ({ default: m.ComparacaoPage })),
);
const FerramentasPage = lazy(() =>
  import('@/ferramentas/FerramentasPage').then((m) => ({ default: m.FerramentasPage })),
);
const CondominiosPage = lazy(() =>
  import('@/condominios/CondominiosPage').then((m) => ({ default: m.CondominiosPage })),
);
const CondominioPage = lazy(() =>
  import('@/condominios/CondominioPage').then((m) => ({ default: m.CondominioPage })),
);
const AdministradorasPage = lazy(() =>
  import('@/condominios/AdministradorasPage').then((m) => ({ default: m.AdministradorasPage })),
);
const GuardaAvancado = lazy(() =>
  import('@/acesso/GuardaAvancado').then((m) => ({ default: m.GuardaAvancado })),
);

const tela = (pagina: ReactNode): ReactNode => <Suspense>{pagina}</Suspense>;

const ROTAS_DE_LISTA: RouteObject[] = [
  { index: true, element: <RotaInicial /> },
  { path: 'skills', element: tela(<SkillsPage />) },
  { path: 'concierge', element: tela(<ConciergePage />) },
  { path: 'ferramentas', element: tela(<FerramentasPage />) },
  { path: 'condominios', element: tela(<CondominiosPage />) },
  { path: 'administradoras', element: tela(<AdministradorasPage />) },
  { path: 'avancado', element: tela(<GuardaAvancado />) },
];

// Detalhe: cabeçalho próprio no lugar da barra global. A aba aberta fica em ?aba=
const ROTAS_DE_DETALHE: RouteObject[] = [
  { path: 'skills/:nome', element: tela(<SkillEditorPage />) },
  { path: 'skills/:nome/:tipo/:id', element: tela(<EditorDaCamadaPage />) },
  { path: 'atualizacoes/:id', element: tela(<ComparacaoPage />) },
  { path: 'condominios/:id', element: tela(<CondominioPage />) },
];

// Rota sem caminho só para isolar erros de tela: o layout continua de pé.
const comLimite = (rotas: RouteObject[]): RouteObject[] => [
  { element: <LimiteDeRota />, children: rotas },
];

export function criarRotas() {
  return createBrowserRouter(
    [
      {
        path: '/',
        // O Copiloto fica acima dos dois layouts: a conversa sobrevive à troca de tela.
        element: (
          <CopilotoProvider>
            <Outlet />
          </CopilotoProvider>
        ),
        errorElement: <ErroDaRaiz />,
        children: [
          { element: <StudioLayout />, children: comLimite(ROTAS_DE_LISTA) },
          { element: <LayoutDeDetalhe />, children: comLimite(ROTAS_DE_DETALHE) },
        ],
      },
    ],
    { basename: '/studio' },
  );
}

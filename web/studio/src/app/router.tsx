import { lazy, type ReactNode, Suspense } from 'react';
import { createBrowserRouter } from 'react-router';
// Nome do arquivo difere de rotaInicial.ts por mais que o caso da letra:
// Windows/macOS têm sistema de arquivos insensível a maiúsculas e os dois
// nomes colidiriam, fazendo o TypeScript resolver o import errado.
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
const FerramentasPage = lazy(() =>
  import('@/ferramentas/FerramentasPage').then((m) => ({ default: m.FerramentasPage })),
);
const CondominiosPage = lazy(() =>
  import('@/condominios/CondominiosPage').then((m) => ({ default: m.CondominiosPage })),
);
const CondominioPage = lazy(() =>
  import('@/condominios/CondominioPage').then((m) => ({ default: m.CondominioPage })),
);
const GuardaAvancado = lazy(() =>
  import('@/acesso/GuardaAvancado').then((m) => ({ default: m.GuardaAvancado })),
);

const tela = (pagina: ReactNode): ReactNode => <Suspense>{pagina}</Suspense>;

export function criarRotas() {
  return createBrowserRouter(
    [
      {
        path: '/',
        element: <StudioLayout />,
        children: [
          { index: true, element: <RotaInicial /> },
          { path: 'skills', element: tela(<SkillsPage />) },
          // A aba aberta fica em ?aba=
          { path: 'skills/:nome', element: tela(<SkillEditorPage />) },
          { path: 'concierge', element: tela(<ConciergePage />) },
          { path: 'ferramentas', element: tela(<FerramentasPage />) },
          { path: 'condominios', element: tela(<CondominiosPage />) },
          { path: 'condominios/:id', element: tela(<CondominioPage />) },
          // A aba aberta fica em ?aba=
          { path: 'avancado', element: tela(<GuardaAvancado />) },
        ],
      },
    ],
    { basename: '/studio' },
  );
}

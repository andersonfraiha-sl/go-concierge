import { lazy, type ReactNode, Suspense } from 'react';
import { createBrowserRouter, Navigate } from 'react-router';
import { StudioLayout } from './StudioLayout';

// Uma tela por chunk: quem abre o Studio baixa só a tela que está vendo.
const AssistentePage = lazy(() =>
  import('@/assistente/AssistentePage').then((m) => ({ default: m.AssistentePage })),
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
          { index: true, element: <Navigate to="/assistente" replace /> },
          { path: 'skills', element: tela(<SkillsPage />) },
          // A aba aberta fica em ?aba=
          { path: 'skills/:nome', element: tela(<SkillEditorPage />) },
          { path: 'assistente', element: tela(<AssistentePage />) },
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

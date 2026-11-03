import { createBrowserRouter, Navigate } from 'react-router';
import { SemAcesso } from '@/acesso/SemAcesso';
import { pode, useEu } from '@/acesso/eu';
import { AssistentePage } from '@/assistente/AssistentePage';
import { AvancadoPage } from '@/avancado/AvancadoPage';
import { CondominioPage } from '@/condominios/CondominioPage';
import { CondominiosPage } from '@/condominios/CondominiosPage';
import { FerramentasPage } from '@/ferramentas/FerramentasPage';
import { SkillEditorPage } from '@/skills/SkillEditorPage';
import { SkillsPage } from '@/skills/SkillsPage';
import { StudioLayout } from './StudioLayout';

export function criarRoteador() {
  return createBrowserRouter(
    [
      {
        path: '/',
        element: <StudioLayout />,
        children: [
          { index: true, element: <Navigate to="/skills" replace /> },
          { path: 'skills', element: <SkillsPage /> },
          // A aba aberta fica em ?aba=
          { path: 'skills/:nome', element: <SkillEditorPage /> },
          { path: 'assistente', element: <AssistentePage /> },
          { path: 'ferramentas', element: <FerramentasPage /> },
          { path: 'condominios', element: <CondominiosPage /> },
          { path: 'condominios/:id', element: <CondominioPage /> },
          // A aba aberta fica em ?aba=
          { path: 'avancado', element: <GuardaAvancado /> },
        ],
      },
    ],
    { basename: '/studio' },
  );
}

function GuardaAvancado() {
  const { data: eu, isPending } = useEu();
  if (isPending) return null;
  return eu && pode(eu, 'studio.avancado') ? <AvancadoPage /> : <SemAcesso />;
}

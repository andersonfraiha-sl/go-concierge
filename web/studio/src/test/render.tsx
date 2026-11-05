import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { type RenderResult, render } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { CopilotoProvider } from '@/copiloto/CopilotoProvider';
import { SnackbarProvider } from '@/shared/snackbar/SnackbarContext';
import { criarTema } from '@/theme/theme';

type Opcoes = {
  /** URL inicial, com query se precisar. */
  rota?: string;
  /** Padrão de rota em que o componente é montado, ex.: "/skills/:nome". */
  caminho?: string;
  /** Layout de rota (com Outlet) em volta da tela, ex.: <LayoutDeDetalhe />. */
  layout?: ReactNode;
};

export function renderComProvedores(
  ui: ReactNode,
  { rota = '/', caminho = '*', layout }: Opcoes = {},
): RenderResult & { user: UserEvent } {
  const cliente = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  // Toda tela fala com o Copiloto (contexto, realce); fechado, ele não faz pedido nenhum.
  const rotaDaTela = layout
    ? {
        element: <CopilotoProvider>{layout}</CopilotoProvider>,
        children: [{ path: caminho, element: ui }],
      }
    : { path: caminho, element: <CopilotoProvider>{ui}</CopilotoProvider> };
  const rotas = createMemoryRouter([rotaDaTela], { initialEntries: [rota] });
  const resultado = render(
    <QueryClientProvider client={cliente}>
      <ThemeProvider theme={criarTema()}>
        <CssBaseline />
        <SnackbarProvider>
          <RouterProvider router={rotas} />
        </SnackbarProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { ...resultado, user: userEvent.setup() };
}
